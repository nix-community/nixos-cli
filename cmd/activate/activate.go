package activate

import (
	"fmt"
	"os"

	"github.com/nix-community/nixos-cli/internal/activation"
	cmdOpts "github.com/nix-community/nixos-cli/internal/cmd/opts"
	cmdUtils "github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/spf13/cobra"
)

func ActivateCommand() *cobra.Command {
	var opts cmdOpts.ActivateOpts

	commands := map[string]string{
		"boot":         "Generate boot entries and make this the default configuration",
		"check":        "Run pre-activation checks and exit",
		"dry-activate": "Show what would be activated but do not perform it",
		"switch":       "Activate this configuration, generate entries, and make this the default configuration",
		"test":         "Activate this configuration, but do not generate boot entries",
	}

	cmd := cobra.Command{
		Use:   "activate <ACTION> [flags]",
		Short: "Run activation scripts for a NixOS system",
		Long:  "Run boot and activation scripts for NixOS generations.",
		Args: func(cmd *cobra.Command, args []string) error {
			if os.Getenv(NIXOS_STC_PARENT_EXE) != "" {
				return nil
			}

			if len(args) == 0 {
				return fmt.Errorf("missing required argument <ACTION>")
			}

			a, err := activation.ParseSwitchToConfigurationAction(args[0])
			if err != nil {
				return err
			}

			opts.Action = a

			return nil
		},
		ValidArgsFunction: func(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
			results := make([]string, 0, len(commands))
			for command, desc := range commands {
				results = append(results, fmt.Sprintf("%v\t%v", command, desc))
			}

			return results, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdUtils.CommandErrorHandler(activateMain(cmd, &opts))
		},
	}

	if os.Getenv("NIXOS_CLI_ATTEMPTING_ACTIVATION") == "" {
		cmd.Flags().StringVarP(&opts.Specialisation, "specialisation", "s", "", "Activate specialisation `name`")
	}

	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Show verbose logging")

	cmdUtils.SetHelpFlagText(&cmd)
	cmd.SetHelpTemplate(cmd.HelpTemplate() + "\nActions:\n" + cmdUtils.AlignedOptions(commands))

	return &cmd
}
