package completion

import (
	"strings"

	"github.com/nix-community/nixos-cli/internal/ssh"
	"github.com/spf13/cobra"
)

func CompleteHost(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	log, cfg := PrepareCompletionResources()

	hosts, err := ssh.GetHosts(cfg.SSH.HostsFileCompletion)
	if err != nil {
		log.Errorf("failed to get hosts: %v", err)
		return []string{}, cobra.ShellCompDirectiveNoFileComp
	}

	// Provide user@host completion
	atIndex := strings.Index(toComplete, "@")
	if atIndex > 0 {
		userHosts := []string{}
		for _, host := range hosts {
			userHosts = append(userHosts, toComplete[:atIndex+1]+host)
		}
		return userHosts, cobra.ShellCompDirectiveNoFileComp
	}

	return hosts, cobra.ShellCompDirectiveNoFileComp
}
