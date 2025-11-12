//go:build linux

package main

import (
	"fmt"
	"os"

	"github.com/nix-community/nixos-cli/internal/activation"
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/system"
	"golang.org/x/sys/unix"
)

const (
	ACTIVATION_SUPERVISOR_LOCK  = "/run/nixos/activation-supervisor.lock"
	ACTIVATION_DONE_SIGNAL_FILE = "/run/nixos/activation-done"
)

func run(opts *Args) error {
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

	// In case the file itself is not removed by the invoking
	// system, make sure it is removed at the end, and also
	// remove it now to prevent the sender from thinking
	// activation is finished prematurely.
	_ = os.RemoveAll(ACTIVATION_DONE_SIGNAL_FILE)
	defer func() {
		_ = os.RemoveAll(ACTIVATION_DONE_SIGNAL_FILE)
	}()

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

	return nil
}

func rollback(s system.System, action activation.SwitchToConfigurationAction, profileName string, generationLink string) error {
	log := s.Logger()

	profileDirectory := generation.GetProfileDirectoryFromName(profileName)

	// Rollback of the system profile should only happen for when an actual
	// generation was created.
	//
	// Otherwise, do not rollback the actual system profile. Just run the previous
	// switch.
	if action == activation.SwitchToConfigurationActionBoot || action == activation.SwitchToConfigurationActionSwitch {
		rollbackArgv := []string{"nix-env", "-p", profileDirectory, "--rollback"}
		rollbackCmd := system.NewCommand(rollbackArgv[0], rollbackArgv[1:]...)

		_, err := s.Run(rollbackCmd)
		if err != nil {
			log.Errorf("failed to run rollback: %v", err)
			return err
		}
	}

	specialisation := ""
	if defaultSpecialisation, err := activation.FindDefaultSpecialisationFromConfig(s, generationLink); err != nil {
		log.Warnf("unable to find default specialisation from config: %v", err)
	} else {
		specialisation = defaultSpecialisation
	}

	if !activation.VerifySpecialisationExists(s, generationLink, specialisation) {
		log.Warnf("specialisation '%v' does not exist", specialisation)
		log.Warn("using base configuration without specialisations")
		specialisation = ""
	}

	if err := activation.SwitchToConfiguration(s, generationLink, action, &activation.SwitchToConfigurationOptions{
		Specialisation: specialisation,
	}); err != nil {
		log.Errorf("failed to run switch-to-configuration: %v", err)
		return err
	}

	return nil
}
