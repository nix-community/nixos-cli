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
	"github.com/spf13/cobra"

	"github.com/nix-community/nixos-cli/internal/build"
)

func ReplCommand() *cobra.Command {
	opts := cmdOpts.ReplOpts{}

	usage := "repl [flags]"
	if build.Flake() {
		usage += " [FLAKE-REF]"
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
			return cobra.NoArgs(cmd, args)
		},
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
  system = import <nixpkgs/nixos> {};
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

	var nixConfig configuration.Configuration
	if opts.FlakeRef != "" {
		nixConfig = configuration.FlakeRefFromString(opts.FlakeRef)
	} else {
		c, err := configuration.FindConfiguration(log, cfg, opts.NixPathIncludes, false)
		if err != nil {
			log.Errorf("failed to find configuration: %v", err)
			return err
		}
		nixConfig = c
	}

	switch c := nixConfig.(type) {
	case *configuration.FlakeRef:
		err := execFlakeRepl(c)
		if err != nil {
			log.Errorf("failed to exec nix flake repl: %v", err)
			return err
		}
	case *configuration.LegacyConfiguration:
		err := execLegacyRepl(c.Includes, os.Getenv("NIXOS_CONFIG") != "")
		if err != nil {
			log.Errorf("failed to exec nix repl: %v", err)
			return err
		}
	}

	return nil
}

func execLegacyRepl(includes []string, impure bool) error {
	motd := formatLegacyMotd()
	expr := fmt.Sprintf(legacyReplExpr, motd)

	argv := []string{"nix", "repl", "--expr", expr}
	for _, v := range includes {
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
