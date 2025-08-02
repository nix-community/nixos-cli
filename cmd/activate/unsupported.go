//go:build !linux

package activate

import (
	"fmt"

	"github.com/nix-community/nixos-cli/internal/activation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/spf13/cobra"
)

func activateMain(cmd *cobra.Command, _ activation.SwitchToConfigurationAction) error {
	log := logger.FromContext(cmd.Context())
	err := fmt.Errorf("the activate command is unsupported on non-NixOS systems")
	log.Error(err)
	return err
}
