package completion

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"github.com/nix-community/nixos-cli/internal/cmd/nixopts"
	"github.com/nix-community/nixos-cli/internal/configuration"
	"github.com/spf13/cobra"
)

// Complete a flake ref that refers to a NixOS configuration.
func CompleteConfigFlakeRef(cobraCmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	uri, system, found := strings.Cut(toComplete, "#")

	if !found {
		// Just try to find flake refs first.
		return CompleteFlakeRefURI(toComplete), cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	}

	// Attempt to complete NixOS system part here.
	cmd := exec.Command("nix", "eval", "--json", "--apply", "builtins.attrNames", fmt.Sprintf("%s#nixosConfigurations", uri))
	output, err := cmd.Output()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var availableSystems []string
	if err = json.Unmarshal(output, &availableSystems); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	var results []string
	for _, v := range availableSystems {
		if strings.HasPrefix(v, system) {
			results = append(results, fmt.Sprintf("%s#%s", uri, v))
		}
	}

	return results, cobra.ShellCompDirectiveNoFileComp
}

// Complete just the URI portion of a flake ref.
// If it contains a system portion, no results
// will be returned.
func CompleteFlakeRefURI(toComplete string) []string {
	if strings.Contains(toComplete, "#") {
		return nil
	}

	candidates, err := nixopts.CollectNixCommandCompletionValues([]string{"build"}, toComplete)
	if err != nil {
		return nil
	}

	results := make([]string, 0, len(candidates))
	for _, v := range candidates {
		results = append(results, v.Value)
	}

	return results
}

// Complete the <ATTR> portion of a positional arguments list,
// assuming that the passed argument is a Nix <FILE> that
// contains attributes that can be evaluated directly and
// correspond to NixOS configurations.
func CompleteNixConfigFileAttr(filename string) ([]string, cobra.ShellCompDirective) {
	configPath, err := configuration.ResolveSystemNix(filename)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	expr := fmt.Sprintf(`builtins.attrNames (import "%s")`, configPath)
	cmd := exec.Command("nix-instantiate", "--json", "--eval", "--expr", expr)
	output, err := cmd.Output()
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	attrs := []string{}
	if err = json.Unmarshal(output, &attrs); err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	return attrs, cobra.ShellCompDirectiveNoFileComp
}
