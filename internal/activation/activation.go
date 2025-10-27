package activation

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/nix-community/nixos-cli/internal/system"
)

// Parse the generation's `nixos-cli` configuration to find the default specialisation
// for that generation.
func FindDefaultSpecialisationFromConfig(s system.System, generationDirname string) (string, error) {
	generationCfgFilename := filepath.Join(generationDirname, constants.DefaultConfigLocation)

	settingsContent, err := s.FS().ReadFile(generationCfgFilename)
	if err != nil {
		return "", err
	}

	generationCfg, err := settings.ParseSettingsFromString(string(settingsContent))
	if err != nil {
		return "", err
	}

	return generationCfg.Apply.DefaultSpecialisation, nil
}

// Make sure a specialisation exists in a given generation and can be activated by
// checking for the presence of the switch-to-configuration script.
func VerifySpecialisationExists(s system.System, generationDirname string, specialisation string) bool {
	if specialisation == "" {
		// The base config always exists.
		return true
	}

	specialisationStcFilename := filepath.Join(generationDirname, "specialisation", specialisation, "bin", "switch-to-configuration")
	if _, err := s.FS().Stat(specialisationStcFilename); err != nil {
		return false
	}

	return true
}

func EnsureSystemProfileDirectoryExists(s system.System) error {
	// The system profile directory sometimes doesn't exist,
	// and does need to be manually created if this is the case.
	// This kinda sucks, since it requires root execution, but
	// there's not really a better way to ensure that this
	// profile's directory exists.

	err := s.FS().MkdirAll(constants.NixSystemProfileDirectory, 0o755)
	if err != nil {
		if err != os.ErrExist {
			return fmt.Errorf("failed to create nix system profile directory: %w", err)
		}
	}

	return nil
}

func AddNewNixProfile(s system.System, profile string, closure string) error {
	if profile != "system" {
		err := EnsureSystemProfileDirectoryExists(s)
		if err != nil {
			return err
		}
	}

	profileDirectory := generation.GetProfileDirectoryFromName(profile)

	argv := []string{"nix-env", "--profile", profileDirectory, "--set", closure}

	s.Logger().CmdArray(argv)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	_, err := s.Run(cmd)
	return err
}

func SetNixProfileGeneration(s system.System, profile string, genNumber uint64) error {
	if profile != "system" {
		err := EnsureSystemProfileDirectoryExists(s)
		if err != nil {
			return err
		}
	}

	profileDirectory := generation.GetProfileDirectoryFromName(profile)

	argv := []string{"nix-env", "--profile", profileDirectory, "--switch-generation", fmt.Sprintf("%d", genNumber)}

	s.Logger().CmdArray(argv)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	_, err := s.Run(cmd)
	return err
}

func GetCurrentGenerationNumber(s system.System, profile string) (uint64, error) {
	genLinkRegex, err := regexp.Compile(fmt.Sprintf(generation.GenerationLinkTemplateRegex, profile))
	if err != nil {
		return 0, fmt.Errorf("failed to compile generation regex: %w", err)
	}

	profileDirectory := generation.GetProfileDirectoryFromName(profile)
	currentGenerationLink, err := s.FS().ReadLink(profileDirectory)
	if err != nil {
		return 0, fmt.Errorf("unable to determine current generation: %v", err)
	}

	if matches := genLinkRegex.FindStringSubmatch(currentGenerationLink); len(matches) > 0 {
		genNumber, err := strconv.ParseInt(matches[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse generation number %v for %v", matches[1], currentGenerationLink)
		}

		return uint64(genNumber), nil
	} else {
		panic("current link format does not match 'profile-generation-link' format")
	}
}

type SwitchToConfigurationAction int

const (
	SwitchToConfigurationActionUnknown = iota
	SwitchToConfigurationActionChecksOnly
	SwitchToConfigurationActionSwitch
	SwitchToConfigurationActionBoot
	SwitchToConfigurationActionTest
	SwitchToConfigurationActionDryActivate
)

func ParseSwitchToConfigurationAction(arg string) (SwitchToConfigurationAction, error) {
	switch arg {
	case "check":
		return SwitchToConfigurationActionChecksOnly, nil
	case "switch":
		return SwitchToConfigurationActionSwitch, nil
	case "boot":
		return SwitchToConfigurationActionBoot, nil
	case "test":
		return SwitchToConfigurationActionTest, nil
	case "dry-activate":
		return SwitchToConfigurationActionDryActivate, nil
	default:
		return SwitchToConfigurationActionUnknown, fmt.Errorf("invalid switch action: %q", arg)
	}
}

func (c SwitchToConfigurationAction) String() string {
	switch c {
	case SwitchToConfigurationActionChecksOnly:
		return "check"
	case SwitchToConfigurationActionSwitch:
		return "switch"
	case SwitchToConfigurationActionBoot:
		return "boot"
	case SwitchToConfigurationActionTest:
		return "test"
	case SwitchToConfigurationActionDryActivate:
		return "dry-activate"
	default:
		panic("unknown switch to configuration action type")
	}
}

type SwitchToConfigurationOptions struct {
	InstallBootloader bool
	Specialisation    string
}

func SwitchToConfiguration(s system.CommandRunner, generationLocation string, action SwitchToConfigurationAction, opts *SwitchToConfigurationOptions) error {
	var commandPath string
	if opts.Specialisation != "" {
		commandPath = filepath.Join(generationLocation, "specialisation", opts.Specialisation, "bin", "switch-to-configuration")
	} else {
		commandPath = filepath.Join(generationLocation, "bin", "switch-to-configuration")
	}

	argv := []string{commandPath, action.String()}

	log := s.Logger()
	if log.GetLogLevel() == logger.LogLevelDebug {
		argv = append(argv, "-v")
	}

	s.Logger().CmdArray(argv)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	if opts.InstallBootloader {
		cmd.SetEnv("NIXOS_INSTALL_BOOTLOADER", "1")
	}

	cmd.SetEnv("NIXOS_CLI_ATTEMPTING_ACTIVATION", "1")
	_, err := s.Run(cmd)
	return err
}
