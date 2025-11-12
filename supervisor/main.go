//go:build linux

package main

import (
	"fmt"
	"os"

	"github.com/nix-community/nixos-cli/internal/activation"
	"github.com/spf13/cobra"
)

type Args struct {
	Action             activation.SwitchToConfigurationAction
	Specialisation     string
	Verbose            bool
	ProfileName        string
	PreviousGeneration string
}

func main() {
	opts := Args{}

	cmd := &cobra.Command{
		Use: "activation-supervisor {switch|boot|test}",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return err
			}

			switch args[0] {
			case "switch":
				opts.Action = activation.SwitchToConfigurationActionSwitch
			case "boot":
				opts.Action = activation.SwitchToConfigurationActionBoot
			case "test":
				opts.Action = activation.SwitchToConfigurationActionTest
			default:
				return fmt.Errorf("expected one of switch|boot|test, got %s", args[0])
			}

			return nil
		},
		ValidArgs: []string{"switch", "boot", "test"},
		Run: func(cmd *cobra.Command, args []string) {
			err := run(&opts)
			if err != nil {
				os.Exit(1)
			}
		},
	}

	cmd.Flags().StringVar(&opts.PreviousGeneration, "previous-gen", "", "Previous generation `path` to roll back to")

	cmd.Flags().StringVarP(&opts.ProfileName, "profile", "p", "system", "System profile `name` to use")
	cmd.Flags().StringVarP(&opts.Specialisation, "specialisation", "s", "", "Activate specialisation `name`")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Show verbose logging")

	_ = cmd.MarkFlagRequired("previous-gen")

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
