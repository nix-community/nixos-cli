package repl

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"github.com/fatih/color"
	"github.com/nix-community/nixos-cli/internal/cmd/nixopts"
	"github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/configuration"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/nix-community/nixos-cli/internal/utils"
	"github.com/spf13/cobra"

	"github.com/nix-community/nixos-cli/internal/build"
)

func ReplCommand() *cobra.Command {
	opts := cmdOpts.ReplOpts{}

	var usage string
	if build.Flake() {
		usage = "repl [FLAKE-REF]"
	} else {
		usage = "repl [FILE] [ATTR]"
	}

	cmd := cobra.Command{
		Use:   usage,
		Short: "Start a Nix REPL with system configuration loaded",
		Long:  "Start a Nix REPL with current system's configuration loaded.",
		Args: func(cmd *cobra.Command, args []string) error {
			if build.Flake() {
				if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
					return err
				}
				if len(args) > 0 {
					opts.FlakeRef = args[0]
				}
				return nil
			}

			if err := cobra.MaximumNArgs(2)(cmd, args); err != nil {
				return err
			}

			if len(args) > 0 {
				opts.File = args[0]
			}

			if len(args) > 1 {
				opts.Attr = args[1]
			}

			return nil
		},
		ValidArgsFunction: cmdUtils.FlakeOrNixFileCompletions,
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdUtils.CommandErrorHandler(replMain(cmd, &opts))
		},
	}

	cmdUtils.SetHelpFlagText(&cmd)

	nixopts.AddIncludesNixOption(&cmd, &opts.NixPathIncludes)

	if build.Flake() {
		cmd.SetHelpTemplate(cmd.HelpTemplate() + `
Arguments:
    [FLAKE-REF]  Flake ref to load attributes from (default: $NIXOS_CONFIG)
`)
	} else {
		cmd.SetHelpTemplate(cmd.HelpTemplate() + `
Arguments:
  [FILE]  File that contains configuration
  [ATTR]  Attribute inside of [FILE] pointing to configuration

  Both arguments are optional. If [FILE] is not specified, then 
  $NIXOS_CONFIG or the 'nixos-config' entry in $NIX_PATH is used. 
  If [ATTR] is not specified, then the top-level attribute of 
  [FILE] is used.
`)
	}

	return &cmd
}

const (
	flakeReplExpr = `let
  flake = builtins.getFlake "%s";
  system = flake.nixosConfigurations."%s";
  motd = ''
%s'';
  scope =
    assert system._type or null == "configuration";
    assert system.class or "nixos" == "nixos";
      system._module.args
      // system._module.specialArgs
      // {
        inherit (system) config options;
        inherit flake;
      };
in
  builtins.seq scope builtins.trace motd scope
`

	legacyReplExpr = `let
%s
  motd = ''
%s'';
in
 builtins.seq system builtins.trace motd system
`

	flakeMotdTemplate = `This Nix REPL has been automatically loaded with a NixOS configuration.

Configuration :: %s

The following values have been added to the toplevel scope:
  - %s :: Flake inputs, outputs, and source information
  - %s :: Configured option values
  - %s :: Option data and associated metadata
  - %s :: %s package set
  - Any additional arguments in %s and %s

Tab completion can be used to browse around all of these attributes.

Use the %s command to reload the configuration after it has
been changed, assuming it is a mutable configuration.

Use %s to see all available repl commands.

%s: %s does not enforce pure evaluation.
`

	legacyMotdTemplate = `This Nix REPL has been automatically loaded with this system's NixOS configuration.

The following values have been added to the toplevel scope:
  - %s :: Configured option values
  - %s :: Option data and associated metadata
  - %s :: %s package set
  - Any additional arguments in %s and %s

Tab completion can be used to browse around all of these attributes.

Use the %s command to reload the configuration after it has
been changed.

Use %s to see all available repl commands.
`
)

func replMain(cmd *cobra.Command, opts *cmdOpts.ReplOpts) error {
	log := logger.FromContext(cmd.Context())
	cfg := settings.FromContext(cmd.Context())

	var nixosConfig configuration.Configuration
	if opts.FlakeRef != "" {
		ref := configuration.FlakeRefFromString(opts.FlakeRef)
		if err := ref.InferSystemFromHostnameIfNeeded(); err != nil {
			log.Errorf("failed to infer hostname: %v", err)
			return err
		}
		nixosConfig = ref
	} else if opts.File != "" {
		configPath, err := utils.ResolveNixFilename(opts.File)
		if err != nil {
			log.Error(err)
			return err
		}

		nixosConfig = &configuration.LegacyConfiguration{
			Includes:        opts.NixPathIncludes,
			ConfigPath:      configPath,
			Attribute:       opts.Attr,
			UseExplicitPath: true,
		}
	} else {
		c, err := configuration.FindConfiguration(log, cfg, opts.NixPathIncludes)
		if err != nil {
			log.Errorf("failed to find configuration: %v", err)
			return err
		}
		nixosConfig = c
	}

	switch c := nixosConfig.(type) {
	case *configuration.FlakeRef:
		err := execFlakeRepl(c)
		if err != nil {
			log.Errorf("failed to exec nix flake repl: %v", err)
			return err
		}
	case *configuration.LegacyConfiguration:
		err := execLegacyRepl(c, os.Getenv("NIXOS_CONFIG") != "")
		if err != nil {
			log.Errorf("failed to exec nix repl: %v", err)
			return err
		}
	}

	return nil
}

func execLegacyRepl(c *configuration.LegacyConfiguration, impure bool) error {
	motd := formatLegacyMotd()

	var systemExpr string
	if c.UseExplicitPath {
		if c.Attribute != "" {
			systemExpr = fmt.Sprintf(`systemFile = import "%s";
  system = systemFile.%s;
`, c.ConfigPath, c.Attribute)
		} else {
			systemExpr = fmt.Sprintf(`system = import "%s";
`, c.ConfigPath)
		}
	} else {
		systemExpr = `system = import <nixpkgs/nixos> {};
`
	}

	expr := fmt.Sprintf(legacyReplExpr, systemExpr, motd)

	argv := []string{"nix", "repl", "--expr", expr}
	for _, v := range c.Includes {
		argv = append(argv, "-I", v)
	}

	if impure {
		argv = append(argv, "--impure")
	}

	nixCommandPath, err := exec.LookPath("nix")
	if err != nil {
		return err
	}

	err = syscall.Exec(nixCommandPath, argv, os.Environ())
	return err
}

func execFlakeRepl(flakeRef *configuration.FlakeRef) error {
	motd := formatFlakeMotd(flakeRef)
	expr := fmt.Sprintf(flakeReplExpr, flakeRef.URI, flakeRef.System, motd)

	argv := []string{"nix", "repl", "--expr", expr}

	nixCommandPath, err := exec.LookPath("nix")
	if err != nil {
		return err
	}

	err = syscall.Exec(nixCommandPath, argv, os.Environ())
	return err
}

func formatFlakeMotd(ref *configuration.FlakeRef) string {
	flakeRef := fmt.Sprintf("%s#%s", ref.URI, ref.System)

	return fmt.Sprintf(flakeMotdTemplate,
		color.CyanString(flakeRef),
		color.MagentaString("flake"),
		color.MagentaString("config"),
		color.MagentaString("options"),
		color.MagentaString("pkgs"), color.CyanString("nixpkgs"),
		color.MagentaString("_module.args"), color.MagentaString("_module.specialArgs"),
		color.MagentaString(":r"),
		color.MagentaString(":?"),
		color.YellowString("warning"), color.CyanString("nixos repl"),
	)
}

func formatLegacyMotd() string {
	return fmt.Sprintf(legacyMotdTemplate,
		color.MagentaString("config"),
		color.MagentaString("options"),
		color.MagentaString("pkgs"), color.CyanString("nixpkgs"),
		color.MagentaString("_module.args"), color.MagentaString("_module.specialArgs"),
		color.MagentaString(":r"),
		color.MagentaString(":?"),
	)
}
