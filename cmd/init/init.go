package init

import (
	"fmt"
	"path/filepath"

	cmdOpts "github.com/nix-community/nixos-cli/internal/cmd/opts"
	cmdUtils "github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/spf13/cobra"
)

func InitCommand() *cobra.Command {
	opts := cmdOpts.InitOpts{}

	cmd := cobra.Command{
		Use:   "init",
		Short: "Initialize a NixOS configuration",
		Long:  "Initialize a NixOS configuration template and/or hardware options.",
		Args: func(cmd *cobra.Command, args []string) error {
			if !filepath.IsAbs(opts.Root) {
				return fmt.Errorf("--root must be an absolute path")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdUtils.CommandErrorHandler(initMain(cmd, &opts))
		},
	}

	cmdUtils.SetHelpFlagText(&cmd)

	cmd.Flags().StringVarP(&opts.Directory, "dir", "d", "/etc/nixos", "Directory `path` in root to write to")
	cmd.Flags().BoolVarP(&opts.ForceWrite, "force", "f", false, "Force generation of all configuration files")
	cmd.Flags().BoolVarP(&opts.NoFSGeneration, "no-fs", "n", false, "Do not generate 'fileSystem' options configuration")
	cmd.Flags().StringVarP(&opts.Root, "root", "r", "/", "Treat `path` as the root directory")
	cmd.Flags().BoolVarP(&opts.ShowHardwareConfig, "show-hardware-config", "s", false, "Print hardware config to stdout and exit")

	_ = cmd.RegisterFlagCompletionFunc("dir", cmdUtils.DirCompletions)
	_ = cmd.RegisterFlagCompletionFunc("root", cmdUtils.DirCompletions)

	return &cmd
}
