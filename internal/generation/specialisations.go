package generation

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	cmdUtils "github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/configuration"
	"github.com/spf13/cobra"
)

func CollectSpecialisations(generationDirname string) ([]string, error) {
	var specialisations []string

	specialisationsGlob := filepath.Join(generationDirname, "specialisation", "*")

	specialisationsMatches, err := filepath.Glob(specialisationsGlob)
	if err != nil {
		return nil, err
	} else {
		for _, match := range specialisationsMatches {
			specialisations = append(specialisations, filepath.Base(match))
		}
	}

	sort.Strings(specialisations)

	return specialisations, nil
}

func CollectSpecialisationsFromConfig(cfg configuration.Configuration) []string {
	var argv []string

	switch c := cfg.(type) {
	case *configuration.FlakeRef:
		attr := fmt.Sprintf("%s#nixosConfigurations.%s.config.specialisation", c.URI, c.System)
		argv = []string{"nix", "eval", attr, "--apply", "builtins.attrNames", "--json"}
	case *configuration.LegacyConfiguration:
		argv = []string{
			"nix-instantiate", "--eval", "--json", "--expr", "builtins.attrNames",
			"builtins.attrNames (import <nixpkgs/nixos> {}).config.specialisation",
		}
	}

	cmd := exec.Command(argv[0], argv[1:]...)

	stdout, err := cmd.Output()
	if err != nil {
		return []string{}
	}

	specialisations := []string{}

	err = json.Unmarshal(stdout, &specialisations)
	if err != nil {
		return []string{}
	}

	return specialisations
}

func CompleteSpecialisationFlag(generationDirname string) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		specialisations, err := CollectSpecialisations(generationDirname)
		if err != nil {
			return []string{}, cobra.ShellCompDirectiveNoFileComp
		}

		candidates := []string{}

		for _, specialisation := range specialisations {
			if specialisation == toComplete {
				return specialisations, cobra.ShellCompDirectiveNoFileComp
			}

			if strings.HasPrefix(specialisation, toComplete) {
				candidates = append(candidates, specialisation)
			}
		}

		return candidates, cobra.ShellCompDirectiveNoFileComp
	}
}

func CompleteSpecialisationFlagFromConfig(flakeRefStr string, includes []string) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		log, cfg := cmdUtils.PrepareCompletionResources()

		var nixConfig configuration.Configuration
		if flakeRefStr != "" {
			nixConfig = configuration.FlakeRefFromString(flakeRefStr)
		} else {
			c, err := configuration.FindConfiguration(log, cfg, includes)
			if err != nil {
				log.Errorf("failed to find configuration: %v", err)
				return []string{}, cobra.ShellCompDirectiveNoFileComp
			}
			nixConfig = c
		}

		if nixConfig == nil {
			log.Error("config is nil")
			return []string{}, cobra.ShellCompDirectiveNoFileComp
		}

		specialisations := CollectSpecialisationsFromConfig(nixConfig)

		candidates := []string{}

		for _, specialisation := range specialisations {
			if specialisation == toComplete {
				return []string{specialisation}, cobra.ShellCompDirectiveNoFileComp
			}

			if strings.HasPrefix(specialisation, toComplete) {
				candidates = append(candidates, specialisation)
			}
		}

		return candidates, cobra.ShellCompDirectiveNoFileComp
	}
}
