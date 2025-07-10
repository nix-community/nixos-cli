package generation

import (
	"fmt"

	"github.com/spf13/cobra"

	cmdOpts "github.com/nix-community/nixos-cli/internal/cmd/opts"
	cmdUtils "github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/system"

	genDeleteCmd "github.com/nix-community/nixos-cli/cmd/generation/delete"
	genDiffCmd "github.com/nix-community/nixos-cli/cmd/generation/diff"
	genListCmd "github.com/nix-community/nixos-cli/cmd/generation/list"
	genRollbackCmd "github.com/nix-community/nixos-cli/cmd/generation/rollback"
	genSwitchCmd "github.com/nix-community/nixos-cli/cmd/generation/switch"
)

func GenerationCommand() *cobra.Command {
	opts := cmdOpts.GenerationOpts{}

	cmd := cobra.Command{
		Use:   "generation {command}",
		Short: "Manage NixOS generations",
		Long:  "Manage NixOS generations on this machine.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			log := logger.FromContext(cmd.Context())
			s := system.NewLocalSystem(log)

			if !s.IsNixOS() {
				return fmt.Errorf("generation commands can only be run on NixOS systems")
			}

			return nil
		},
	}

	cmd.PersistentFlags().StringVarP(&opts.ProfileName, "profile", "p", "system", "System profile to use")

	cmd.AddCommand(genDeleteCmd.GenerationDeleteCommand(&opts))
	cmd.AddCommand(genDiffCmd.GenerationDiffCommand(&opts))
	cmd.AddCommand(genListCmd.GenerationListCommand(&opts))
	cmd.AddCommand(genSwitchCmd.GenerationSwitchCommand(&opts))
	cmd.AddCommand(genRollbackCmd.GenerationRollbackCommand(&opts))

	cmdUtils.SetHelpFlagText(&cmd)

	_ = cmd.RegisterFlagCompletionFunc("profile", generation.CompleteProfileFlag)

	return &cmd
}
