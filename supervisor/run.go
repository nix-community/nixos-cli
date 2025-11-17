//go:build linux

package main

import (
	"fmt"
	"os"

	"github.com/nix-community/nixos-cli/internal/activation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/spf13/cobra"
)

const (
	ACTIVATION_SUPERVISOR_LOCK = "/run/nixos/activation-supervisor.lock"
)

type RunArgs struct {
	Action             activation.SwitchToConfigurationAction
	Specialisation     string
	Verbose            bool
	ProfileName        string
	PreviousGeneration string
}

func RunCommand() *cobra.Command {
	opts := RunArgs{}

	cmd := &cobra.Command{
		Use:   "run {switch|boot|test}",
		Short: "Run switch-to-configuration script and start rollback watchdog",
		Long:  "Run switch-to-configuration script and start rollback watchdog.",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return err
			}

			switch args[0] {
			case "switch":
				opts.Action = activation.SwitchToConfigurationActionSwitch
			case "boot":
				opts.Action = activation.SwitchToConfigurationActionBoot
			case "test":
				opts.Action = activation.SwitchToConfigurationActionTest
			default:
				return fmt.Errorf("expected one of switch|boot|test, got %s", args[0])
			}

			return nil
		},
		ValidArgs: []string{"switch", "boot", "test"},
		Run: func(cmd *cobra.Command, args []string) {
			err := runMain(cmd, &opts)
			if err != nil {
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&opts.PreviousGeneration, "previous-gen", "", "Previous generation `path` to roll back to")

	cmd.Flags().StringVarP(&opts.ProfileName, "profile", "p", "system", "System profile `name` to use")
	cmd.Flags().StringVarP(&opts.Specialisation, "specialisation", "s", "", "Activate specialisation `name`")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Show verbose logging")

	_ = cmd.MarkFlagRequired("previous-gen")

	return cmd
}

// Execute the watchdog command using systemd-run.
//
// This ensures that even if the activation process itself is killed,
// that the watchdog continues to run.
//
// Once the process starts up, this returns.
func execWatchdog(s system.System, opts *RunArgs) error {
	log := s.Logger()

	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	argv := []string{
		"systemd-run",
		"-E", RUNNING_ACTIVATION_SUPERVISOR,
		"-E", "LOCALE_ARCHIVE",
		"-E", "NIXOS_INSTALL_BOOTLOADER",
		"-E", "TOPLEVEL",
		"--collect",
		"--no-ask-password",
		"--no-block",
		"--quiet",
		"--service-type=exec",
		"--unit=nixos-cli-activation-supervisor-watchdog",
		exePath, "watchdog", opts.Action.String(),
		"--previous-gen", opts.PreviousGeneration,
	}
	if opts.ProfileName != "system" {
		argv = append(argv, "-p", opts.ProfileName)
	}
	if opts.Specialisation != "" {
		argv = append(argv, "-s", opts.Specialisation)
	}
	if opts.Verbose {
		argv = append(argv, "-v")
	}

	log.CmdArray(argv)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	_, err = s.Run(cmd)
	return err
}

func runMain(cmd *cobra.Command, opts *RunArgs) error {
	log := logger.FromContext(cmd.Context())

	if syslogLogger, err := logger.NewSyslogLogger("nixos-cli-activation-supervisor"); err == nil {
		log = logger.NewMultiLogger(log, syslogLogger)
	} else {
		log.Warnf("failed to initialize syslog logger: %v", err)
	}
	if opts.Verbose {
		log.SetLogLevel(logger.LogLevelDebug)
	}

	s := system.NewLocalSystem(log)

	unlockProcess, err := acquireProcessLock(log, ACTIVATION_SUPERVISOR_LOCK)
	if err != nil {
		return err
	}
	defer unlockProcess()

	toplevel := os.Getenv("TOPLEVEL")
	if toplevel == "" {
		err := fmt.Errorf("$TOPLEVEL is not set")
		log.Error(err)
		return err
	}

	specialisation := opts.Specialisation
	if specialisation == "" {
		if defaultSpecialisation, err := activation.FindDefaultSpecialisationFromConfig(s, toplevel); err != nil {
			log.Warnf("unable to find default specialisation from config: %v", err)
		} else {
			specialisation = defaultSpecialisation
		}
	}

	if !activation.VerifySpecialisationExists(s, toplevel, specialisation) {
		log.Warnf("specialisation '%v' does not exist", specialisation)
		log.Warn("using base configuration without specialisations")
		specialisation = ""
	}

	if err := activation.SwitchToConfiguration(s, toplevel, opts.Action, &activation.SwitchToConfigurationOptions{
		InstallBootloader: false,
		Specialisation:    specialisation,
	}); err != nil {
		log.Errorf("failed to run switch-to-configuration: %v", err)

		origErr := err
		if err := rollback(s, opts.Action, opts.ProfileName, opts.PreviousGeneration); err != nil {
			return origErr
		}
		return err
	}

	err = execWatchdog(s, opts)
	if err != nil {
		log.Errorf("failed to run watchdog: %v", err)
		return err
	}

	return nil
}
