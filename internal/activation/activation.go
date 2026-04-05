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
	UseRootCommand bool
	RootElevator   *system.RootElevator
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
		cmd.AsRoot(opts.RootElevator)
	}

	_, err := s.Run(cmd)
	return err
}

type SetNixProfileGenerationOptions struct {
	UseRootCommand bool
	RootElevator   *system.RootElevator
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
		cmd.AsRoot(opts.RootElevator)
	}

	_, err := s.Run(cmd)
	return err
}

type RollbackNixProfileOptions struct {
	UseRootCommand bool
	RootElevator   *system.RootElevator
}

func RollbackNixProfile(s system.System, profile string, opts *RollbackNixProfileOptions) error {
	if profile != "system" {
		err := EnsureSystemProfileDirectoryExists(s)
		if err != nil {
			return err
		}
	}

	profileDirectory := generation.GetProfileDirectoryFromName(profile)

	argv := []string{"nix-env", "--profile", profileDirectory, "--rollback"}

	s.Logger().CmdArray(argv)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	if opts.UseRootCommand {
		cmd.AsRoot(opts.RootElevator)
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
	RootElevator      *system.RootElevator
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
		cmd.AsRoot(opts.RootElevator)
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
	RootElevator      *system.RootElevator
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
) (err error) {
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
		cmd.AsRoot(opts.RootElevator)
	}

	// If the root elevator uses TTY input, then debug logs will
	// not work since the terminal will be forced into raw mode.
	//
	// As such, any debug logging during the ACK process must be
	// delayed and replayed later.
	//
	// This is a rather niche edge case, but it is useful nonetheless
	// for users of `doas` and other root elevators that require
	// TTY input.
	supervisorLogger := log
	if opts.RootElevator != nil && opts.RootElevator.Method == settings.PasswordInputMethodTTY {
		log.Warn("terminal is in raw mode; will replay some supervisor logs after completion")
		replayLogger := logger.NewReplayLogger(log)
		defer func() {
			if err != nil && replayLogger.HasEntries() {
				log.Print("--- LOG OUTPUT DURING ACTIVATION: ---")
				replayLogger.Flush()
			}
		}()
		supervisorLogger = replayLogger
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
		case activationErr := <-activationComplete:
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

			if _, ok := activationErr.(*ssh.ExitMissingError); ok {
				log.Warn("lost connection to target host, attempting to reconnect")
				activationErr = s.(*system.SSHSystem).Reconnect()
				if activationErr != nil {
					log.Errorf("%v", activationErr)
					log.Info("the target host should rollback soon")
					return activationErr
				}

				log.Debug("attempting to acknowledge success after reconnecting")
				if activationErr = s.FS().CreateFile(successTrigger); activationErr != nil {
					log.Errorf("failed to create %v on remote system: %v", successTrigger, activationErr)
					log.Info("the target host should rollback soon")
					return activationErr
				}
				return nil
			}

			// At this point, the SSH command has failed to run and
			// we cannot do anything more, so exit with an error.
			if activationErr != nil {
				return fmt.Errorf("activation supervisor exited early: %w", activationErr)
			}
			return errors.New("activation supervisor exited without success signal")
		case <-successTriggerCheckTimer.C:
			// Check for the existence of the switch success trigger.
			// If it exists, then the switch has completed
			// and we can proceed to signaling the watchdog.
			_, statErr := s.FS().Stat(SWITCH_SUCCESS_PATH)
			if statErr == nil {
				successDetected = true
				break
			}
			if errors.Is(statErr, os.ErrNotExist) {
				continue
			}
			return fmt.Errorf("failed checking success file: %w", statErr)
		}
	}

	// Create a new target host connection, since SSH sessions
	// will not terminate if sshd itself has terminated.
	reconnectCh := make(chan *system.SSHSystem)

	go func() {
		supervisorLogger.Debug("attempting reconnect")
		s2, reconnectErr := s.(*system.SSHSystem).Clone()
		if reconnectErr != nil {
			supervisorLogger.Errorf("%v", reconnectErr)
			supervisorLogger.Warnf("it is very likely that SSH access cannot be re-established")
			supervisorLogger.Warnf("the target host should rollback soon")
			close(reconnectCh)
			return
		}

		select {
		case reconnectCh <- s2:
			// Do nothing, the connection has been re-established here.
			break
		case <-time.After(opts.AckTimeout):
			// Otherwise, close the connection since no other
			// goroutine will pick it up.
			s2.Close()
		}
	}()

	select {
	case s2 := <-reconnectCh:
		if s2 == nil {
			break
		}

		defer s2.Close()
		createErr := s2.FS().CreateFile(successTrigger)
		if createErr != nil {
			supervisorLogger.Errorf("failed to create %v on remote system: %v", successTrigger, createErr)
		}
	case <-time.After(opts.AckTimeout):
		// Once the timeout is hit, the reconnect goroutine
		// will automatically close the connection since it
		// wasn't received here.
		break
	}

	supervisorLogger.Debug("waiting for activation process to complete")

	if !activationChConsumed {
		// Wait for the watchdog to either rollback or finish.
		// If the SSH connection is broken, then this case
		// cannot be hit because the connection was
		// already terminated with a broken pipe.
		activationErr := <-activationComplete
		if activationErr != nil {
			return fmt.Errorf("activation supervisor exited with error: %w", activationErr)
		}
	}

	return nil
}
