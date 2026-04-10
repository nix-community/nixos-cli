package completion

import (
	"os"

	"github.com/nix-community/nixos-cli/internal/build"
	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/spf13/cobra"
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

// Completions for commands that require <FLAKE> || <FILE> <ATTR>
// arguments, depending on if the application is built with
// flakes or not.
func FlakeOrNixFileCompletions(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if build.Flake() {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return CompleteConfigFlakeRef(cmd, args, toComplete)
	}

	switch len(args) {
	case 0:
		return FileCompletions("nix")(cmd, args, toComplete)
	case 1:
		// We are now completing <ATTR> for legacy mode.
		return CompleteNixConfigFileAttr(args[0])
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func DirCompletions(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	return nil, cobra.ShellCompDirectiveFilterDirs
}

func FileCompletions(extensions ...string) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		if len(extensions) != 0 {
			return extensions, cobra.ShellCompDirectiveFilterFileExt
		} else {
			return nil, cobra.ShellCompDirectiveDefault
		}
	}
}
