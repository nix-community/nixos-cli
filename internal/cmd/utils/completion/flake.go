package completion

import (
	"github.com/nix-community/nixos-cli/internal/configuration"
	"github.com/spf13/cobra"
)

func CompleteConfigFlakeRef(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	currentRef := configuration.FlakeRefFromString(toComplete)

	if currentRef.System != "" {
		// Attempt to complete system here.
	} else {
		// Just try to find flake refs first!
	}

	return nil, cobra.ShellCompDirectiveDefault
}
