package activate

import (
	"fmt"

	cmdOpts "github.com/nix-community/nixos-cli/internal/cmd/opts"
	cmdUtils "github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/spf13/cobra"
)

func ActivateCommand() *cobra.Command {
	opts := cmdOpts.ActivateOpts{}

	cmd := cobra.Command{
		Use:   "activate [flags] [OPTION-NAME]",
		Short: "Run activation scripts for a NixOS system",
		Long:  "Run boot and activation scripts for NixOS generations.",
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if !opts.Activate && !opts.CreateBootEntries && !opts.RunChecksOnly {
				return fmt.Errorf("at least one action must be specified")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdUtils.CommandErrorHandler(activateMain(cmd, &opts))
		},
	}

	cmd.Flags().BoolVarP(&opts.Activate, "activate", "a", false, "Activate the configuration now")
	cmd.Flags().BoolVarP(&opts.CreateBootEntries, "boot", "b", false, "Regenerate boot entries and set as default")
	cmd.Flags().BoolVarP(&opts.RunChecksOnly, "checks-only", "c", false, "Run pre-switch checks and exit")
	cmd.Flags().BoolVarP(&opts.Dry, "dry", "d", false, "Show what would be done but do not perform it")

	return &cmd
}
