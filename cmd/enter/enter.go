package enter

import (
	"github.com/spf13/cobra"

	cmdOpts "github.com/nix-community/nixos-cli/internal/cmd/opts"
	cmdUtils "github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/logger"
)

func EnterCommand() *cobra.Command {
	opts := cmdOpts.EnterOpts{}

	cmd := cobra.Command{
		Use:   "enter [flags] [-- ARGS...]",
		Short: "Chroot into a NixOS installation",
		Long:  "Enter a NixOS chroot environment.",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 {
				opts.CommandArray = args
			}

			return nil
		},
		PreRun: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			log := logger.FromContext(ctx)

			if opts.Verbose {
				log.SetLogLevel(logger.LogLevelDebug)
			}

			ctx = logger.WithLogger(ctx, log)
			cmd.SetContext(ctx)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdUtils.CommandErrorHandler(enterMain(cmd, &opts))
		},
	}

	cmd.Flags().StringVarP(&opts.Command, "command", "c", "", "Command `string` to execute in bash")
	cmd.Flags().StringVarP(&opts.RootLocation, "root", "r", "/mnt", "NixOS system root `path` to enter")
	cmd.Flags().StringVar(&opts.System, "system", "", "NixOS system configuration to activate at `path`")
	cmd.Flags().BoolVarP(&opts.Silent, "silent", "s", false, "Suppress all system activation output")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Show verbose logging")

	_ = cmd.RegisterFlagCompletionFunc("root", cmdUtils.DirCompletions)
	_ = cmd.RegisterFlagCompletionFunc("system", cmdUtils.DirCompletions)

	cmd.MarkFlagsMutuallyExclusive("silent", "verbose")

	cmdUtils.SetHelpFlagText(&cmd)
	cmd.SetHelpTemplate(cmd.HelpTemplate() + `
Arguments:
  [ARGS...]  Interpret arguments as the command to run directly

If providing a command through positional arguments with flags, a preceding
double dash (--) is required. Otherwise, unexpected behavior may occur.
`)

	return &cmd
}
