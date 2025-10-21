package diff

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/nix-community/nixos-cli/internal/system"
)

func GenerationDiffCommand(genOpts *cmdOpts.GenerationOpts) *cobra.Command {
	opts := cmdOpts.GenerationDiffOpts{}

	cmd := cobra.Command{
		Use:   "diff {BEFORE} {AFTER}",
		Short: "Show what changed between two generations",
		Long:  "Display what paths differ between two generations.",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(2)(cmd, args); err != nil {
				return err
			}

			before, err := strconv.ParseInt(args[0], 10, 32)
			if err != nil {
				return fmt.Errorf("{BEFORE} must be an integer, got '%v'", before)
			}
			opts.Before = uint(before)

			after, err := strconv.ParseInt(args[1], 10, 32)
			if err != nil {
				return fmt.Errorf("{AFTER} must be an integer, got '%v'", after)
			}
			opts.After = uint(after)

			return nil
		},
		ValidArgsFunction: generation.CompleteGenerationNumber(&genOpts.ProfileName, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdUtils.CommandErrorHandler(generationDiffMain(cmd, genOpts, &opts))
		},
	}

	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Show verbose logging")

	cmd.SetHelpTemplate(cmd.HelpTemplate() + `
Arguments:
  [BEFORE]  Number of first generation to compare with
  [AFTER]   Number of second generation to compare with
`)
	cmdUtils.SetHelpFlagText(&cmd)

	return &cmd
}

func generationDiffMain(cmd *cobra.Command, genOpts *cmdOpts.GenerationOpts, opts *cmdOpts.GenerationDiffOpts) error {
	log := logger.FromContext(cmd.Context())
	cfg := settings.FromContext(cmd.Context())
	s := system.NewLocalSystem(log)

	profileDirectory := constants.NixProfileDirectory
	if genOpts.ProfileName != "system" {
		profileDirectory = constants.NixSystemProfileDirectory
	}

	beforeDirectory := filepath.Join(profileDirectory, fmt.Sprintf("%v-%v-link", genOpts.ProfileName, opts.Before))
	afterDirectory := filepath.Join(profileDirectory, fmt.Sprintf("%v-%v-link", genOpts.ProfileName, opts.After))

	err := generation.RunDiffCommand(s, beforeDirectory, afterDirectory, &generation.DiffCommandOptions{
		UseNvd:  cfg.UseNvd,
		Verbose: opts.Verbose,
	})
	if err != nil {
		log.Errorf("failed to run diff command: %v", err)
		return err
	}

	return nil
}
