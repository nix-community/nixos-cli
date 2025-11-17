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
	WATCHDOG_LOCK = "/run/nixos/activation-watchdog.lock"
)

type WatchdogArgs struct {
	Action             activation.SwitchToConfigurationAction
	Specialisation     string
	Verbose            bool
	ProfileName        string
	PreviousGeneration string
}

func WatchdogCommand() *cobra.Command {
	opts := WatchdogArgs{}

	cmd := &cobra.Command{
		Use:   "watchdog {switch|boot|test}",
		Short: "Watch for an acknowledgement signal from the deployer",
		Long: `Watch for an acknowledgement signal from the deployer,
and rollback the NixOS configuration if a canary file is not
created to signal success within a period of time.`,
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
			err := watchdogMain(cmd, &opts)
			if err != nil {
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&opts.PreviousGeneration, "previous-gen", "", "Previous generation `path` to roll back to")
	cmd.Flags().StringVarP(&opts.ProfileName, "profile", "p", "system", "System profile `name` to use")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Show verbose logging")

	_ = cmd.MarkFlagRequired("previous-gen")

	return cmd
}

func watchdogMain(cmd *cobra.Command, opts *WatchdogArgs) error {
	log := logger.FromContext(cmd.Context())

	if syslogLogger, err := logger.NewSyslogLogger("nixos-cli-activation-watchdog"); err == nil {
		log = logger.NewMultiLogger(log, syslogLogger)
	} else {
		log.Warnf("failed to initialize syslog logger: %v", err)
	}
	if opts.Verbose {
		log.SetLogLevel(logger.LogLevelDebug)
	}

	s := system.NewLocalSystem(log)

	unlockProcess, err := acquireProcessLock(log, WATCHDOG_LOCK)
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

	return nil
}
