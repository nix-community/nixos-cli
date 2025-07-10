//go:build !linux

package enter

import (
	"fmt"

	cmdOpts "github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/spf13/cobra"
)

func enterMain(cmd *cobra.Command, _ *cmdOpts.EnterOpts) error {
	log := logger.FromContext(cmd.Context())
	err := fmt.Errorf("the enter command is unsupported on non-Linux systems")
	log.Error(err)
	return err
}
