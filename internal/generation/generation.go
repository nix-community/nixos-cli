package generation

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/djherbis/times"
	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/logger"
)

func GetProfileDirectoryFromName(profile string) string {
	if profile != "system" {
		return filepath.Join(constants.NixSystemProfileDirectory, profile)
	} else {
		return filepath.Join(constants.NixProfileDirectory, "system")
	}
}

type Generation struct {
	Number          uint64    `json:"number"`
	CreationDate    time.Time `json:"creation_date"`
	IsCurrent       bool      `json:"is_current"`
	KernelVersion   string    `json:"kernel_version"`
	Specialisations []string  `json:"specialisations"`

	NixosVersion          string `json:"nixos_version"`
	NixpkgsRevision       string `json:"nixpkgs_revision"`
	ConfigurationRevision string `json:"configuration_revision"`
	Description           string `json:"description"`
}

type GenerationManifest struct {
	NixosVersion          string `json:"nixosVersion"`
	NixpkgsRevision       string `json:"nixpkgsRevision"`
	ConfigurationRevision string `json:"configurationRevision"`
	Description           string `json:"description"`
}

type GenerationReadError struct {
	Directory string
	Number    uint64
	Errors    []error
}

func (e *GenerationReadError) Error() string {
	return fmt.Sprintf("failed to read generation %d from directory %s", e.Number, e.Directory)
}

func GenerationFromDirectory(generationDirname string, number uint64) (*Generation, error) {
	info := &Generation{
		Number:          number,
		CreationDate:    time.Time{},
		IsCurrent:       false,
		KernelVersion:   "",
		Specialisations: []string{},
	}

	nixosVersionManifestFile := filepath.Join(generationDirname, "nixos-version.json")

	encounteredErrors := []error{}

	manifestBytes, err := os.ReadFile(nixosVersionManifestFile)
	if err != nil {
		// The `nixos-version.json` file does not exist in generations that
		// are created without the corresponding NixOS module enabled or
		// created with `nixos-rebuild`/other application tools, and should
		// be ignored.
		if !errors.Is(err, os.ErrNotExist) {
			encounteredErrors = append(encounteredErrors, err)
		}
	} else {
		var manifest GenerationManifest
		err := json.Unmarshal(manifestBytes, &manifest)

		if err != nil {
			encounteredErrors = append(encounteredErrors, err)
		} else {
			info.NixosVersion = manifest.NixosVersion
			info.NixpkgsRevision = manifest.NixpkgsRevision
			info.ConfigurationRevision = manifest.ConfigurationRevision
			info.Description = manifest.Description
		}
	}

	// Fall back to reading the nixos-version file that should always
	// exist if the `nixos-version.json` file doesn't exist.
	if info.NixosVersion == "" {
		nixosVersionFile := filepath.Join(generationDirname, "nixos-version")
		nixosVersionContents, err := os.ReadFile(nixosVersionFile)
		if err != nil {
			encounteredErrors = append(encounteredErrors, err)
		} else {
			info.NixosVersion = string(nixosVersionContents)
		}
	}

	// Get time of creation for the generation
	creationTimeStat, err := times.Stat(generationDirname)
	if err != nil {
		encounteredErrors = append(encounteredErrors, err)
	} else {
		if creationTimeStat.HasBirthTime() {
			info.CreationDate = creationTimeStat.BirthTime()
		} else {
			info.CreationDate = creationTimeStat.ModTime()
		}
	}

	kernelVersionDirGlob := filepath.Join(generationDirname, "kernel-modules", "lib", "modules", "*")
	kernelVersionMatches, err := filepath.Glob(kernelVersionDirGlob)
	if err != nil {
		encounteredErrors = append(encounteredErrors, err)
	} else if len(kernelVersionMatches) == 0 {
		encounteredErrors = append(encounteredErrors, fmt.Errorf("no kernel modules version directory found"))
	} else {
		info.KernelVersion = filepath.Base(kernelVersionMatches[0])
	}

	specialisations, err := CollectSpecialisations(generationDirname)
	if err != nil {
		encounteredErrors = append(encounteredErrors, err)
	}

	info.Specialisations = specialisations

	if len(encounteredErrors) > 0 {
		return info, &GenerationReadError{
			Directory: generationDirname,
			Number:    number,
			Errors:    encounteredErrors,
		}
	}

	return info, nil
}

const (
	GenerationLinkTemplateRegex = `^%s-(\d+)-link$`
)

func CollectGenerationsInProfile(log *logger.Logger, profile string) ([]Generation, error) {
	profileDirectory := constants.NixProfileDirectory
	if profile != "system" {
		profileDirectory = constants.NixSystemProfileDirectory
	}

	generationDirEntries, err := os.ReadDir(profileDirectory)
	if err != nil {
		return nil, err
	}

	genLinkRegex, err := regexp.Compile(fmt.Sprintf(GenerationLinkTemplateRegex, profile))
	if err != nil {
		return nil, fmt.Errorf("failed to compile generation regex: %w", err)
	}

	currentGenerationDirname := GetProfileDirectoryFromName(profile)
	currentGenerationLink, err := os.Readlink(currentGenerationDirname)
	if err != nil {
		log.Warnf("unable to determine current generation: %v", err)
	}

	generations := []Generation{}
	for _, v := range generationDirEntries {
		name := v.Name()

		if matches := genLinkRegex.FindStringSubmatch(name); len(matches) > 0 {
			genNumber, err := strconv.ParseInt(matches[1], 10, 64)
			if err != nil {
				log.Warnf("failed to parse generation number %v for %v, skipping", matches[1], filepath.Join(profileDirectory, name))
				continue
			}

			profileDirectory := constants.NixProfileDirectory
			if profile != "system" {
				profileDirectory = constants.NixSystemProfileDirectory
			}

			generationDirectoryName := filepath.Join(profileDirectory, fmt.Sprintf("%s-%d-link", profile, genNumber))

			info, err := GenerationFromDirectory(generationDirectoryName, uint64(genNumber))
			if err != nil {
				return nil, err
			}

			if name == currentGenerationLink {
				info.IsCurrent = true
			}

			generations = append(generations, *info)
		}
	}

	sort.Slice(generations, func(i, j int) bool {
		return generations[i].Number < generations[j].Number
	})

	return generations, nil
}
