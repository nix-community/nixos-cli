package activation

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/nix-community/nixos-cli/internal/system"
	"golang.org/x/crypto/ssh"
)

const (
	ACTIVATION_LOCKFILE = "/run/nixos/switch-to-configuration.lock"
	SWITCH_SUCCESS_PATH = "/run/nixos/switch-success"
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

func IsNixOSClosure(s system.System, closure string) (bool, error) {
	nixosVersionFile := filepath.Join(closure, constants.NixOSVersionFile)

	_, err := s.FS().Stat(nixosVersionFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	return true, nil
}

var ErrNixosClosureMissingFiles = `
Your NixOS closure path seems to be missing essential files.
To avoid corrupting your current NixOS installation, the activation will abort.

This could be caused by a Nix bug: https://github.com/NixOS/nix/issues/13367.
This is the evaluated NixOS closure path: %v.
Change the directory to somewhere else (e.g., 'cd $HOME') before trying again.

Please open an issue if you think this is a mistake.`

type AddNewNixProfileOptions struct {
	RootCommand    string
	UseRootCommand bool
}

func AddNewNixProfile(s system.System, profile string, closure string, opts *AddNewNixProfileOptions) error {
	if profile != "system" {
		err := EnsureSystemProfileDirectoryExists(s)
		if err != nil {
			return err
		}
	}

	if isClosure, err := IsNixOSClosure(s, closure); !isClosure {
		if err == nil {
			return fmt.Errorf(ErrNixosClosureMissingFiles, closure)
		}

		return err
	}

	profileDirectory := generation.GetProfileDirectoryFromName(profile)

	argv := []string{"nix-env", "--profile", profileDirectory, "--set", closure}

	s.Logger().CmdArray(argv)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	if opts.UseRootCommand {
		cmd.AsRoot(opts.RootCommand)
	}

	_, err := s.Run(cmd)
	return err
}

type SetNixProfileGenerationOptions struct {
	RootCommand    string
	UseRootCommand bool
}

func SetNixProfileGeneration(s system.System, profile string, genNumber uint64, opts *SetNixProfileGenerationOptions) error {
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
	if opts.UseRootCommand {
		cmd.AsRoot(opts.RootCommand)
	}

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
		var genNumber uint64
		genNumber, err = strconv.ParseUint(matches[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse generation number %v for %v", matches[1], currentGenerationLink)
		}

		return genNumber, nil
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
	UseRootCommand    bool
	RootCommand       string
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
	if opts.UseRootCommand {
		cmd.AsRoot(opts.RootCommand)
	}

	if opts.InstallBootloader {
		cmd.SetEnv("NIXOS_INSTALL_BOOTLOADER", "1")
	}

	cmd.SetEnv("NIXOS_CLI_ATTEMPTING_ACTIVATION", "1")

	cmd.InheritEnv("NIXOS_NO_CHECK")

	_, err := s.Run(cmd)
	return err
}

// Create an activation trigger path name from a NixOS system
// closure's location.
//
// Used for remote activation.
func MakeActivationTriggerPath(systemLocation string) string {
	// Obtain the cryptographic hash + nixos system closure name
	basename := filepath.Base(systemLocation)

	hash, _, found := strings.Cut(basename, "-")
	if !found {
		// Use the SHA256 hash of the whole path if the hash
		// part in the filename is not found. This should be
		// rare, if it happens at all, so this ensures collisions
		// do not happen most of the time.
		hashedBasename := sha256.Sum256([]byte(systemLocation))
		hash = hex.EncodeToString(hashedBasename[:])
	}

	return filepath.Join(constants.NixOSActivationDirectory, "trigger", hash)
}

// Create the activation runtime directories with the required
// structure.
//
// This creates the trigger directory as sticky, in case non-root users
// need to activate things.
//
// NOTE: this trigger directory is world-writable to enable non-root users
// to create files in it. This will need to be revisited if the ACK
// trigger needs to be secure in the future.
func EnsureActivationDirectoryExists() error {
	err := os.MkdirAll(constants.NixOSActivationDirectory, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create %s: %s", constants.NixOSActivationDirectory, err)
	}

	triggerDirectory := filepath.Join(constants.NixOSActivationDirectory, "trigger")

	err = os.MkdirAll(triggerDirectory, 0o777|os.ModeSticky)
	if err != nil {
		return fmt.Errorf("failed to create %s: %s", triggerDirectory, err)
	}

	err = os.Chmod(triggerDirectory, 0o777|os.ModeSticky)
	if err != nil {
		return fmt.Errorf("failed to set permissions for %s: %s", triggerDirectory, err)
	}

	return nil
}

type RunActivationSupervisorOptions struct {
	ProfileName       string
	InstallBootloader bool
	Specialisation    string
	UseRootCommand    bool
	RootCommand       string
	AckTimeout        time.Duration

	PreviousSpecialisation   string
	RollbackProfileOnFailure bool
}

//go:embed supervisor.sh
var activationSupervisorScript string

func RunActivationSupervisor(
	s system.System,
	systemLocation string,
	action SwitchToConfigurationAction,
	opts *RunActivationSupervisorOptions,
) error {
	if _, ok := s.(*system.SSHSystem); !ok {
		panic("RunActivationSupervisor() called with a non-SSH system")
	}

	log := s.Logger()

	argv := []string{
		"systemd-run",
		"--collect",
		"--no-ask-password",
		"--pipe",
		"--quiet",
		"--service-type=exec",
		"--unit=nixos-cli-activation-supervisor",
		"--wait",
		"-E", "PATH",
		"-E", fmt.Sprintf("TOPLEVEL=%s", systemLocation),
		"-E", fmt.Sprintf("ACTION=%s", action.String()),
	}

	if opts.Specialisation != "" {
		argv = append(argv, "-E", fmt.Sprintf("SPECIALISATION=%s", opts.Specialisation))
	}

	if opts.PreviousSpecialisation != "" {
		argv = append(argv, "-E", fmt.Sprintf("PREVIOUS_SPECIALISATION=%s", opts.PreviousSpecialisation))
	}

	if opts.ProfileName != "" {
		profileDirectory := generation.GetProfileDirectoryFromName(opts.ProfileName)
		argv = append(argv, "-E", fmt.Sprintf("PROFILE=%s", profileDirectory))
	}

	successTrigger := MakeActivationTriggerPath(systemLocation)
	argv = append(argv, "-E", fmt.Sprintf("ACK_TRIGGER_PATH=%s", successTrigger))

	if opts.RollbackProfileOnFailure {
		argv = append(argv, "-E", "ROLLBACK_PROFILE_ON_FAILURE=1")
	}

	argv = append(argv, "-E", "LOCALE_ARCHIVE")

	if opts.InstallBootloader {
		argv = append(argv, "-E", "NIXOS_INSTALL_BOOTLOADER=1")
	}

	if log.GetLogLevel() == logger.LogLevelDebug {
		argv = append(argv, "-E", "VERBOSE=1")
	}

	ackTimeout := int(opts.AckTimeout / time.Second)
	if ackTimeout < 1 {
		return fmt.Errorf("acknowledgement timeout must be at least 1 second")
	}
	argv = append(argv, "-E", fmt.Sprintf("ACK_TIMEOUT=%d", ackTimeout))

	argv = append(argv, "/bin/sh", "-c", activationSupervisorScript)

	if os.Getenv("NIXOS_CLI_DEBUG_MODE") != "" {
		log.CmdArray(argv)
	} else {
		displayArgv := append([]string(nil), argv[0:len(argv)-1]...)
		displayArgv = append(displayArgv, "<ACTIVATION-SCRIPT>")
		log.CmdArray(displayArgv)
	}

	cmd := system.NewCommand(argv[0], argv[1:]...)
	if opts.UseRootCommand {
		cmd.AsRoot(opts.RootCommand)
	}

	activationComplete := make(chan error, 1)
	go func() {
		// FIXME: there are times where if a connection is taken
		// down using `ip link set down`, then the whole process
		// hangs. Figure out a way to detect this condition.
		_, activationErr := s.Run(cmd)
		activationComplete <- activationErr
	}()

	successTriggerCheckTimer := time.NewTicker(500 * time.Millisecond)
	defer successTriggerCheckTimer.Stop()

	successDetected := false
	activationChConsumed := false
	for !successDetected {
		select {
		case err := <-activationComplete:
			// Check one more time if a success trigger has been created, in case
			// the activation error returned before the ticker.
			// This should be almost impossible, but just in case it happens, the
			// final check is here.
			if _, statErr := s.FS().Stat(SWITCH_SUCCESS_PATH); statErr == nil {
				successDetected = true
				activationChConsumed = true
				break
			}

			// At this point, the supervisor exited before the success file appeared,
			// so the switch has either failed, or a transport error has occurred
			// and we need to re-initiate the SSH connection.

			if _, ok := err.(*ssh.ExitMissingError); ok {
				log.Warn("lost connection to target host, attempting to reconnect")
				err = s.(*system.SSHSystem).Reconnect()
				if err != nil {
					log.Errorf("%v", err)
					log.Info("the target host should rollback soon")
					return err
				}

				log.Debug("attempting to acknowledge success after reconnecting")
				if err = s.FS().CreateFile(successTrigger); err != nil {
					log.Errorf("failed to create %v on remote system: %v", successTrigger, err)
					log.Info("the target host should rollback soon")
					return err
				}
				return nil
			}

			// At this point, the SSH command has failed to run and
			// we cannot do anything more, so exit with an error.
			if err != nil {
				return fmt.Errorf("activation supervisor exited early: %w", err)
			}
			return errors.New("activation supervisor exited without success signal")
		case <-successTriggerCheckTimer.C:
			// Check for the existence of the switch success trigger.
			// If it exists, then the switch has completed
			// and we can proceed to signaling the watchdog.
			_, err := s.FS().Stat(SWITCH_SUCCESS_PATH)
			if err == nil {
				successDetected = true
				break
			}
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("failed checking success file: %w", err)
		}
	}

	// Create a new target host connection, since SSH sessions
	// will not terminate if sshd itself has terminated.
	reconnectCh := make(chan *system.SSHSystem, 1)

	go func() {
		log.Debug("attempting reconnect")
		s2, err := s.(*system.SSHSystem).Clone()
		if err != nil {
			log.Errorf("%v", err)
			log.Warnf("it is very likely that SSH access cannot be re-established")
			log.Warnf("the target host should rollback soon")
			reconnectCh <- nil
			return
		}
		reconnectCh <- s2
	}()

	select {
	case s2 := <-reconnectCh:
		if s2 == nil {
			break
		}

		defer s2.Close()
		err := s2.FS().CreateFile(successTrigger)
		if err != nil {
			log.Errorf("failed to create %v on remote system: %v", successTrigger, err)
		}
	case <-time.After(opts.AckTimeout):
		// FIXME: fix race condition where s2 is never closed if this is reached first.
		break
	}

	log.Debug("waiting for activation process to complete")

	if !activationChConsumed {
		// Wait for the watchdog to either rollback or finish.
		// If the SSH connection is broken, then this case
		// cannot be hit because the connection was
		// already terminated with a broken pipe.
		err := <-activationComplete
		if err != nil {
			return fmt.Errorf("activation supervisor exited with error: %v", err)
		}
	}

	return nil
}
