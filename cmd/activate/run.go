//go:build linux

package activate

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"github.com/nix-community/nixos-cli/internal/activation"
	cmdOpts "github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"
)

type RequiredVars struct {
	OutPath              string
	Toplevel             string
	PreSwitchCheckCmd    string
	InstallBootloaderCmd string
	LocaleArchive        string
	NewSystemd           string
}

type ErrorRequiredVarMissing struct {
	VarName string
}

func (e ErrorRequiredVarMissing) Error() string {
	return fmt.Sprintf("missing required environment variable $%s, this is a bug", e.VarName)
}

func getRequiredVars() (*RequiredVars, error) {
	outPath := os.Getenv("OUT")
	if outPath == "" {
		return nil, ErrorRequiredVarMissing{VarName: "OUT"}
	}

	toplevel := os.Getenv("TOPLEVEL")
	if toplevel == "" {
		return nil, ErrorRequiredVarMissing{VarName: "TOPLEVEL"}
	}

	preSwitchCheck := os.Getenv("PRE_SWITCH_CHECK")
	if preSwitchCheck == "" {
		return nil, ErrorRequiredVarMissing{VarName: "PRE_SWITCH_CHECK"}
	}

	installBootloaderCmd := os.Getenv("INSTALL_BOOTLOADER_CMD")
	if installBootloaderCmd == "" {
		return nil, ErrorRequiredVarMissing{VarName: "INSTALL_BOOTLOADER_CMD"}
	}

	localeArchive := os.Getenv("LOCALE_ARCHIVE")
	if localeArchive == "" {
		return nil, ErrorRequiredVarMissing{VarName: "LOCALE_ARCHIVE"}
	}

	newSystemd := os.Getenv("SYSTEMD")
	if newSystemd == "" {
		return nil, ErrorRequiredVarMissing{VarName: "SYSTEMD"}
	}

	return &RequiredVars{
		OutPath:              outPath,
		Toplevel:             toplevel,
		PreSwitchCheckCmd:    preSwitchCheck,
		InstallBootloaderCmd: installBootloaderCmd,
		LocaleArchive:        localeArchive,
		NewSystemd:           newSystemd,
	}, nil
}

func execInSwitchContext(
	s system.CommandRunner,
	log *logger.Logger,
	action activation.SwitchToConfigurationAction,
	specialisation string,
) error {
	if specialisation != "" {
		specialisations, err := generation.CollectSpecialisations(constants.CurrentSystem)
		if err != nil {
			log.Warnf("unable to access specialisations: %v", err)
		}

		if !slices.Contains(specialisations, specialisation) {
			err = fmt.Errorf("specialisation '%v' does not exist", specialisations)
			log.Error(err)
			return err
		}
	}

	err := activation.SwitchToConfiguration(s, constants.CurrentSystem, action, &activation.SwitchToConfigurationOptions{
		Specialisation: specialisation,
	})

	return err
}

func runPreSwitchCheck(
	s system.CommandRunner,
	cmdStr string,
	toplevel string,
	action activation.SwitchToConfigurationAction,
) error {
	// TODO: would it be more appropriate to use shlex.Split() here?
	args := strings.Split(cmdStr, " ")
	args = append(args, toplevel)
	args = append(args, action.String())

	cmd := system.NewCommand(args[0], args[1:]...)
	_, err := s.Run(cmd)
	return err
}

const (
	// TODO: this can maybe change in the future?
	ACTIVATION_LOCKFILE = "/run/nixos/switch-to-configuration.lock"
)

func activateMain(cmd *cobra.Command, opts *cmdOpts.ActivateOpts) error {
	log := logger.FromContext(cmd.Context())
	s := system.NewLocalSystem(log)

	if attemptingActivation := os.Getenv("NIXOS_CLI_ATTEMPTING_ACTIVATION"); attemptingActivation == "" {
		err := execInSwitchContext(s, log, opts.Action, opts.Specialisation)
		if err != nil {
			log.Errorf("failed to re-execute switch-to-configuration script: %v", err)
		}

		return err
	}

	if !s.IsNixOS() {
		err := fmt.Errorf("the activate command is unsupported on non-NixOS systems")
		log.Error(err)
		return err
	}

	vars, err := getRequiredVars()
	if err != nil {
		log.Errorf("%s", err)
		return err
	}

	err = os.Setenv("NIXOS_ACTION", opts.Action.String())
	if err != nil {
		log.Errorf("failed to set NIXOS_ACTION variable: %s", err)
		return err
	}

	err = os.Setenv("LOCALE_ARCHIVE", vars.LocaleArchive)
	if err != nil {
		log.Errorf("failed to set LOCALE_ARCHIVE variable: %s", err)
		return err
	}

	err = os.MkdirAll("/run/nixos", 0o755)
	if err != nil {
		log.Errorf("failed to create /run/nixos: %s", err)
		return err
	}

	lockfile, err := os.OpenFile(ACTIVATION_LOCKFILE, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		log.Errorf("failed to create activation lockfile %s: %s", ACTIVATION_LOCKFILE, err)
		return err
	}
	defer func() { _ = lockfile.Close() }()

	if err := unix.Flock(int(lockfile.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		log.Errorf("failed to lock %s", ACTIVATION_LOCKFILE)
		log.Info("is another activation process running?")
		return err
	}
	defer unix.Flock(int(lockfile.Fd()), unix.LOCK_UN)

	// TODO: syslog init?

	if skipCheck := os.Getenv("NIXOS_NO_CHECK"); skipCheck == "" {
		log.Info("running pre-switch checks")

		err = runPreSwitchCheck(s, vars.PreSwitchCheckCmd, vars.Toplevel, opts.Action)
		if err != nil {
			log.Errorf("failed to run pre-switch check commands: %s", err)
			return err
		}
	}

	if opts.Action == activation.SwitchToConfigurationActionChecksOnly {
		return nil
	}

	return nil
}
