package root

import (
	"context"
	"fmt"
	"os"

	"github.com/carapace-sh/carapace"
	"github.com/fatih/color"
	"github.com/nix-community/nixos-cli/internal/build"
	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/spf13/cobra"

	"github.com/nix-community/nixos-cli/internal/cmd/opts"

	activateCmd "github.com/nix-community/nixos-cli/cmd/activate"
	applyCmd "github.com/nix-community/nixos-cli/cmd/apply"
	completionCmd "github.com/nix-community/nixos-cli/cmd/completion"
	enterCmd "github.com/nix-community/nixos-cli/cmd/enter"
	featuresCmd "github.com/nix-community/nixos-cli/cmd/features"
	generationCmd "github.com/nix-community/nixos-cli/cmd/generation"
	infoCmd "github.com/nix-community/nixos-cli/cmd/info"
	initCmd "github.com/nix-community/nixos-cli/cmd/init"
	installCmd "github.com/nix-community/nixos-cli/cmd/install"
	manualCmd "github.com/nix-community/nixos-cli/cmd/manual"
	optionCmd "github.com/nix-community/nixos-cli/cmd/option"
	replCmd "github.com/nix-community/nixos-cli/cmd/repl"
)

const helpTemplate = `Usage:{{if .Runnable}}
  {{.UseLine}}{{end}}{{if .HasAvailableSubCommands}}
  {{.CommandPath}} [command]{{end}}{{if gt (len .Aliases) 0}}

Aliases:
  {{.NameAndAliases}}{{end}}{{if .HasExample}}

Examples:
{{.Example}}{{end}}{{if .HasAvailableSubCommands}}{{$cmds := .Commands}}{{if eq (len .Groups) 0}}

Commands:{{range $cmds}}{{if (or .IsAvailableCommand (eq .Name "help"))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{else}}{{if not .AllChildCommandsHaveGroup}}

Commands:{{range $cmds}}{{if (and (eq .GroupID "") (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{range $group := .Groups}}

{{.Title}}:{{range $cmds}}{{if (and (eq .GroupID $group.ID) (or .IsAvailableCommand (eq .Name "help")))}}
  {{rpad .Name .NamePadding }} {{.Short}}{{end}}{{end}}{{end}}{{end}}{{end}}{{if .HasAvailableLocalFlags}}

Flags:
{{.LocalFlags.FlagUsages | trimTrailingWhitespaces}}{{end}}
`

func mainCommand() (*cobra.Command, error) {
	opts := cmdOpts.MainOpts{}

	log := logger.NewConsoleLogger()
	cmdCtx := logger.WithLogger(context.Background(), log)

	configLocation := os.Getenv("NIXOS_CLI_CONFIG")
	if configLocation == "" {
		configLocation = constants.DefaultConfigLocation
	}

	cfg, err := settings.ParseSettings(configLocation)
	if err != nil {
		if os.Getenv("NIXOS_CLI_SUPPRESS_NO_SETTINGS_WARNING") == "" && !carapace.IsCallback() {
			log.Error(err)
			log.Warn("proceeding with defaults only, you have been warned")
		}

		cfg = settings.NewSettings()
	}

	errs := cfg.Validate()
	for _, err := range errs {
		log.Warn(err.Error())
	}

	cmdCtx = settings.WithConfig(cmdCtx, cfg)

	cmd := cobra.Command{
		Use:                        "nixos {command} [flags]",
		Short:                      "nixos-cli",
		Long:                       "A tool for managing NixOS installations",
		Version:                    build.Version(),
		SilenceUsage:               true,
		SuggestionsMinimumDistance: 1,
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			for key, value := range opts.ConfigValues {
				err := cfg.SetValue(key, value)
				if err != nil {
					return fmt.Errorf("failed to set %v: %w", key, err)
				}
			}

			errs := cfg.Validate()
			for _, err := range errs {
				log.Warn(err.Error())
			}

			// Now that we have the real color settings from parsing
			// the configuration and command-line arguments, set it.
			//
			// Precedence of color settings:
			// 1. -C flag -> true
			// 2. NO_COLOR=1 -> false, fatih/color already takes this into account
			// 3. `color` setting from config (default: true)
			if opts.ColorAlways {
				color.NoColor = false
				log.RefreshColorPrefixes()
			} else if os.Getenv("NO_COLOR") == "" {
				color.NoColor = !cfg.UseColor
				log.RefreshColorPrefixes()
			}

			return nil
		},
	}

	cmd.SetContext(cmdCtx)

	cmd.SetHelpCommand(&cobra.Command{Hidden: true})
	cmd.SetUsageTemplate(helpTemplate)

	boldRed := color.New(color.FgRed).Add(color.Bold)
	cmd.SetErrPrefix(boldRed.Sprint("error:"))

	cmd.Flags().BoolP("help", "h", false, "Show this help menu")
	cmd.Flags().BoolP("version", "v", false, "Display version information")

	cmd.PersistentFlags().BoolVar(&opts.ColorAlways, "color-always", false, "Always color output when possible")
	cmd.PersistentFlags().StringToStringVar(&opts.ConfigValues, "config", map[string]string{}, "Set a configuration `key=value`")

	_ = cmd.RegisterFlagCompletionFunc("config", settings.CompleteConfigFlag)

	cmd.AddCommand(activateCmd.ActivateCommand())
	cmd.AddCommand(applyCmd.ApplyCommand(cfg))
	cmd.AddCommand(completionCmd.CompletionCommand())
	cmd.AddCommand(enterCmd.EnterCommand())
	cmd.AddCommand(featuresCmd.FeatureCommand())
	cmd.AddCommand(generationCmd.GenerationCommand())
	cmd.AddCommand(infoCmd.InfoCommand())
	cmd.AddCommand(initCmd.InitCommand())
	cmd.AddCommand(installCmd.InstallCommand())
	cmd.AddCommand(manualCmd.ManualCommand())
	cmd.AddCommand(optionCmd.OptionCommand())
	cmd.AddCommand(replCmd.ReplCommand())

	for alias, resolved := range cfg.Aliases {
		err := addAliasCmd(&cmd, alias, resolved)
		if err != nil {
			log.Warnf("failed to add alias '%v': %v", alias, err.Error())
		}
	}

	carapace.Gen(cmd.Root())

	return &cmd, nil
}

func Execute() {
	cmd, err := mainCommand()
	if err != nil {
		os.Exit(1)
	}

	if err = cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
