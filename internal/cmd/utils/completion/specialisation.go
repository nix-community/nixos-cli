package completion

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/nix-community/nixos-cli/internal/configuration"
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/spf13/cobra"
)

func CollectSpecialisationsFromConfig(cfg configuration.Configuration) []string {
	var argv []string

	switch c := cfg.(type) {
	case *configuration.FlakeRef:
		attr := c.ConfigAttr("specialisation")
		argv = []string{"nix", "eval", attr, "--apply", "builtins.attrNames", "--json"}
	case *configuration.LegacyConfiguration:
		configPathArg := c.ConfigPathArg()
		if configPathArg == "<nixpkgs/nixos>" {
			configPathArg += " {}"
		}
		argv = []string{
			"nix-instantiate", "--eval", "--json", "--expr",
			fmt.Sprintf(`builtins.attrNames (import "%s").%s`, configPathArg, c.ConfigAttr("specialisation")),
		}
		for _, include := range c.Includes {
			argv = append(argv, "-I", include)
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
		log, _ := PrepareCompletionResources()
		s := system.NewLocalSystem(log)

		specialisations, err := generation.CollectSpecialisations(s, generationDirname)
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

type configResolver func(cmd *cobra.Command, args []string) configuration.Configuration

func CompleteSpecialisationFlagFromConfig(resolver configResolver) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		nixConfig := resolver(cmd, args)

		if nixConfig == nil {
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
