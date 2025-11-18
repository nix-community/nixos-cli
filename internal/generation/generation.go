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
	"github.com/nix-community/nixos-cli/internal/system"
)

func GetProfileDirectoryFromName(profile string) string {
	if profile != "system" {
		return filepath.Join(constants.NixSystemProfileDirectory, profile)
	} else {
		return filepath.Join(constants.NixProfileDirectory, "system")
	}
}

type Generation struct {
	Path            string    `json:"path"`
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

func GenerationFromDirectory(s system.System, generationDirname string, number uint64) (*Generation, error) {
	generationPath, err := s.FS().RealPath(generationDirname)
	if err != nil {
		generationPath = generationDirname
	}

	info := &Generation{
		Path:            generationPath,
		Number:          number,
		CreationDate:    time.Time{},
		IsCurrent:       false,
		KernelVersion:   "",
		Specialisations: []string{},
	}

	nixosVersionManifestFile := filepath.Join(generationDirname, "nixos-version.json")

	encounteredErrors := []error{}

	manifestBytes, err := s.FS().ReadFile(nixosVersionManifestFile)
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

		if err = json.Unmarshal(manifestBytes, &manifest); err != nil {
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
		nixosVersionFile := filepath.Join(generationDirname, constants.NixOSVersionFile)

		var nixosVersionContents []byte
		nixosVersionContents, err = s.FS().ReadFile(nixosVersionFile)

		if err != nil {
			encounteredErrors = append(encounteredErrors, err)
		} else {
			info.NixosVersion = string(nixosVersionContents)
		}
	}

	// Get time of creation for the generation
	switch s.(type) {
	case *system.LocalSystem:
		var creationTimeStat times.Timespec
		creationTimeStat, err = times.Stat(generationDirname)
		if err != nil {
			encounteredErrors = append(encounteredErrors, err)
		} else {
			if creationTimeStat.HasBirthTime() {
				info.CreationDate = creationTimeStat.BirthTime()
			} else {
				info.CreationDate = creationTimeStat.ModTime()
			}
		}
	}

	kernelVersionDirGlob := filepath.Join(generationDirname, "kernel-modules", "lib", "modules", "*")
	kernelVersionMatches, err := s.FS().Glob(kernelVersionDirGlob)
	if err != nil {
		encounteredErrors = append(encounteredErrors, err)
	} else if len(kernelVersionMatches) == 0 {
		encounteredErrors = append(encounteredErrors, fmt.Errorf("no kernel modules version directory found"))
	} else {
		info.KernelVersion = filepath.Base(kernelVersionMatches[0])
	}

	specialisations, err := CollectSpecialisations(s, generationDirname)
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

func CollectGenerationsInProfile(s system.System, log logger.Logger, profile string) ([]Generation, error) {
	profileDirectory := constants.NixProfileDirectory
	if profile != "system" {
		profileDirectory = constants.NixSystemProfileDirectory
	}

	generationDirEntries, err := s.FS().ReadDir(profileDirectory)
	if err != nil {
		return nil, err
	}

	var genLinkRegex *regexp.Regexp
	genLinkRegex, err = regexp.Compile(fmt.Sprintf(GenerationLinkTemplateRegex, profile))
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
			var genNumber uint64
			genNumber, err = strconv.ParseUint(matches[1], 10, 64)
			if err != nil {
				log.Warnf("failed to parse generation number %v for %v, skipping", matches[1], filepath.Join(profileDirectory, name))
				continue
			}

			generationDirectoryName := filepath.Join(profileDirectory, fmt.Sprintf("%s-%d-link", profile, genNumber))

			var info *Generation
			info, err = GenerationFromDirectory(s, generationDirectoryName, genNumber)
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
