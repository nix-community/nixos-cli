//go:build !linux

package init

import (
	"fmt"

	cmdOpts "github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/spf13/cobra"
)

func initMain(cmd *cobra.Command, _ *cmdOpts.InitOpts) error {
	log := logger.FromContext(cmd.Context())
	err := fmt.Errorf("the init command is unsupported on non-Linux systems")
	log.Error(err)
	return err
}
