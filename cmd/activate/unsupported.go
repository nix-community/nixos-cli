//go:build !linux

package activate

import (
	"fmt"

	cmdOpts "github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/spf13/cobra"
)

func activateMain(cmd *cobra.Command, _ *cmdOpts.ActivateOpts) error {
	log := logger.FromContext(cmd.Context())
	err := fmt.Errorf("the activate command is unsupported on non-NixOS systems")
	log.Error(err)
	return err
}
