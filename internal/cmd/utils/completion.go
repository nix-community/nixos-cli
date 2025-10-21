package cmdUtils

import (
	"os"

	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
)

// Prepare command resources that are needed for completion, but that
// otherwise need to be retrieved from the Cobra command context.
//
// Only for use in carapace completion functions.
func PrepareCompletionResources() (logger.Logger, *settings.Settings) {
	var log logger.Logger
	if debugMode := os.Getenv("NIXOS_CLI_DEBUG_MODE"); debugMode != "" {
		log = logger.NewConsoleLogger()
	} else {
		log = logger.NewNoOpLogger()
	}

	configLocation := os.Getenv("NIXOS_CLI_CONFIG")
	if configLocation == "" {
		configLocation = constants.DefaultConfigLocation
	}

	cfg, err := settings.ParseSettings(configLocation)
	if err != nil {
		cfg = settings.NewSettings()
	}

	return log, cfg
}
