//go:build linux

package main

import (
	"os"

	"github.com/nix-community/nixos-cli/internal/build"
	"github.com/spf13/cobra"
)

func mainCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "activation-supervisor",
		Short:        "activation-supervisor",
		Long:         "nixos-cli activation supervisor for activating remote systems.",
		Version:      build.Version(),
		SilenceUsage: true,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	cmd.AddCommand(RunCommand())
	cmd.SetHelpCommand(&cobra.Command{Hidden: true})

	cmd.SetHelpTemplate(cmd.HelpTemplate() + `
This command is not meant to be ran directly. Consult nixos-cli-apply(1) for
more information on how this is executed.
`)

	return cmd
}

func main() {
	if err := mainCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
