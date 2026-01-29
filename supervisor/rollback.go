package main

import (
	"github.com/nix-community/nixos-cli/internal/activation"
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/system"
)

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
