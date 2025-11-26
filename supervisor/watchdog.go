//go:build linux

package main

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/nix-community/nixos-cli/internal/activation"
	"github.com/nix-community/nixos-cli/internal/constants"
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

func watchdogMain(cmd *cobra.Command, opts *WatchdogArgs) (err error) {
	// No need to use the syslogger here, since it is ran in a transient
	// systemd service.
	log := logger.FromContext(cmd.Context())
	if opts.Verbose {
		log.SetLogLevel(logger.LogLevelDebug)
	}

	s := system.NewLocalSystem(log)

	unlockProcess, err := acquireProcessLock(log, WATCHDOG_LOCK)
	if err != nil {
		return
	}
	defer unlockProcess()

	toplevel := os.Getenv("TOPLEVEL")
	if toplevel == "" {
		err = fmt.Errorf("$TOPLEVEL is not set")
		log.Error(err)
		return
	}

	triggerDirectory := filepath.Join(constants.NixOSActivationDirectory, "trigger")
	triggerPath := activation.MakeActivationTriggerPath(toplevel)

	defer func() {
		if err != nil {
			log.Info("rolling back configuration to %s", opts.PreviousGeneration)
			err = rollback(s, opts.Action, opts.ProfileName, opts.PreviousGeneration)
			if err != nil {
				log.Errorf("failed to rollback configuration: %v", err)
				log.Info("you are on your own!")
			}
		}
	}()

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Errorf("failed to create file watcher: %v", err)
		return
	}
	defer func() { _ = watcher.Close() }()

	if err = watcher.Add(triggerDirectory); err != nil {
		log.Errorf("failed to add trigger path to file watcher: %s", err)
		return
	}

	// Always remove the file before and afterwards, to prevent
	// false positives.
	_ = os.RemoveAll(triggerPath)
	defer func() {
		_ = os.RemoveAll(triggerPath)
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer func() {
		signal.Stop(sigCh)
		close(sigCh)
	}()

	rollbackTimer := time.After(30 * time.Second)

	log.Debugf("waiting for file %s to get created", triggerPath)

waitloop:
	for {
		select {
		case c := <-sigCh:
			err = fmt.Errorf("caught signal %s", c.String())
			log.Error(err)
			break waitloop
		case <-rollbackTimer:
			err = fmt.Errorf("timeout expired and no acknowledgement was received")
			log.Error(err)
			break waitloop
		case event, ok := <-watcher.Events:
			if !ok {
				return err
			}

			if event.Name == triggerPath && event.Has(fsnotify.Create) {
				log.Debug("acknowledgement received, rollback is not necessary")
				break waitloop
			}
		}
	}

	return err
}
