//go:build linux

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/nix-community/nixos-cli/internal/activation"
	"github.com/nix-community/nixos-cli/internal/build"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/spf13/cobra"
)

const (
	RUNNING_ACTIVATION_SUPERVISOR = "NIXOS_CLI_RUNNING_ACTIVATION_SUPERVISOR"
)

func mainCommand() *cobra.Command {
	log := logger.NewConsoleLogger()

	cmdCtx := logger.WithLogger(context.Background(), log)

	cmd := &cobra.Command{
		Use:          "activation-supervisor",
		Short:        "activation-supervisor",
		Long:         "nixos-cli activation supervisor for activating remote systems.",
		Version:      build.Version(),
		SilenceUsage: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			s := system.NewLocalSystem(log)

			if !s.IsNixOS() {
				return fmt.Errorf("the activation supervisor is not supported on non-NixOS systems")
			}

			if os.Geteuid() != 0 {
				return fmt.Errorf("this command must be ran as root")
			}

			if os.Getenv(RUNNING_ACTIVATION_SUPERVISOR) != "1" {
				return fmt.Errorf("the activation supervisor is not meant to be ran directly by users")
			}

			if err := activation.EnsureActivationDirectoryExists(); err != nil {
				return err
			}

			return nil
		},
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
	}

	cmd.AddCommand(RunCommand())
	cmd.AddCommand(WatchdogCommand())

	cmd.SetHelpCommand(&cobra.Command{Hidden: true})
	cmd.SetHelpTemplate(cmd.HelpTemplate() + `
This command is not meant to be ran directly. Consult nixos-cli-apply(1) for
more information on how this is executed.
`)

	cmd.SetContext(cmdCtx)

	return cmd
}

func main() {
	if err := mainCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
