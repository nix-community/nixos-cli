//go:build linux

package main

import (
	"fmt"
	"os"

	"github.com/nix-community/nixos-cli/internal/activation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
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
			err := runMain(&opts)
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

func runMain(opts *RunArgs) error {
	var log logger.Logger = logger.NewConsoleLogger()
	if syslogLogger, err := logger.NewSyslogLogger("nixos-cli-activation-supervisor"); err == nil {
		log = logger.NewMultiLogger(log, syslogLogger)
	} else {
		log.Warnf("failed to initialize syslog logger: %v", err)
	}
	if opts.Verbose {
		log.SetLogLevel(logger.LogLevelDebug)
	}

	s := system.NewLocalSystem(log)

	if !s.IsNixOS() {
		err := fmt.Errorf("the activation supervisor is not supported on non-NixOS systems")
		log.Error(err)
		return err
	}

	if os.Geteuid() != 0 {
		err := fmt.Errorf("this command must be ran as root")
		log.Errorf("%s", err)
		return err
	}

	if os.Getenv("NIXOS_CLI_RUNNING_ACTIVATION_SUPERVISOR") != "1" {
		err := fmt.Errorf("the activation supervisor is not meant to be ran directly by users")
		log.Error(err)
		return err
	}

	err := os.MkdirAll("/run/nixos", 0o755)
	if err != nil {
		log.Errorf("failed to create /run/nixos: %s", err)
		return err
	}

	lockfile, err := os.OpenFile(ACTIVATION_SUPERVISOR_LOCK, os.O_CREATE|os.O_RDONLY, 0o600)
	if err != nil {
		log.Errorf("failed to create activation lockfile %s: %s", ACTIVATION_SUPERVISOR_LOCK, err)
		return err
	}
	defer func() {
		_ = lockfile.Close()
		_ = os.Remove(ACTIVATION_SUPERVISOR_LOCK)
	}()

	if err := unix.Flock(int(lockfile.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		log.Errorf("failed to lock %s", ACTIVATION_SUPERVISOR_LOCK)
		log.Info("is another activation supervisor process running?")
		return err
	}
	defer func() { _ = unix.Flock(int(lockfile.Fd()), unix.LOCK_UN) }()

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

	// TODO: exec watchdog cmd

	return nil
}
