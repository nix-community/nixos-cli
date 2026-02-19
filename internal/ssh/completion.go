package ssh

import (
	cmdUtils "github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/spf13/cobra"
)

func CompleteHost(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	_, cfg := cmdUtils.PrepareCompletionResources()

	hosts, err := getHosts(cfg.SSH.HostsFileCompletion)
	if err != nil {
		return []string{}, cobra.ShellCompDirectiveNoFileComp
	}

	return hosts, cobra.ShellCompDirectiveNoFileComp
}
