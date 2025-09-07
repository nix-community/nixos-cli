//go:build linux

package activate

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/nix-community/nixos-cli/internal/activation"
	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/spf13/cobra"
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

func execInSwitchContext(s system.CommandRunner, log *logger.Logger, action activation.SwitchToConfigurationAction) error {
	// TODO: document behavior and what is required for this to be set properly.
	specialisation := os.Getenv("NIXOS_SPECIALISATION")

	if specialisation != "" {
		specialisations, err := generation.CollectSpecialisations(constants.CurrentSystem)
		if err != nil {
			log.Warnf("unable to access specialisations: %v", err)
		}

		if !slices.Contains(specialisations, specialisation) {
			log.Errorf("specialisation defined in $NIXOS_SPECIALISATION is %v does not exist")
			return err
		}
	}

	err := activation.SwitchToConfiguration(s, constants.CurrentSystem, action, &activation.SwitchToConfigurationOptions{
		Specialisation: specialisation,
	})

	return err
}

func activateMain(cmd *cobra.Command, action activation.SwitchToConfigurationAction) error {
	log := logger.FromContext(cmd.Context())
	s := system.NewLocalSystem(log)

	if attemptingActivation := os.Getenv("NIXOS_CLI_ATTEMPTING_ACTIVATION"); attemptingActivation == "" {
		err := execInSwitchContext(s, log, action)
		log.Errorf("failed to re-execute switch-to-configuration script: %v", err)
		return err
	}

	if !s.IsNixOS() {
		err := fmt.Errorf("the activate command is unsupported on non-NixOS systems")
		log.Error(err)
		return err
	}

	vars, err := getRequiredVars()
	if err != nil {
		return err
	}

	bytes, _ := json.MarshalIndent(vars, "", "  ")
	fmt.Printf("%s\n", string(bytes))

	// TODO: run pre-switch checks

	return nil
}
