NIXOS-CLI-ENV(5)

# NAME

nixos-cli-env - environment variables used by *nixos-cli*

# DESCRIPTION

The *nixos-cli* tool is influenced by a number of environment variables. These
allow the user to configure the CLI's behavior at runtime, such as:

- Default inputs
- Formatting preferences

Among other things.

# ENVIRONMENT VARIABLES

*NO_COLOR*
	Disable colored output in *nixos-cli*.

	Does not apply to terminal user interfaces (TUIs), which manage their own
	display logic.

*NIXOS_CLI_CONFIG*
	Specify a custom path for the *nixos-cli* settings file. 

	Default: */etc/nixos-cli/config.toml*

*NIXOS_CONFIG*
	Defines the configuration that *nixos-cli* will operate on.
	Its meaning depends on whether the CLI is built with flake support.

	This environment variable takes precedence over the *config_location*
	setting if it is set.

	*Flake-enabled CLIs:*
		This must be a valid flake ref (e.g., */home/user/config* or
		*github:myorg/nixos-config#attr*).

		Flake refs without a local path may have slightly different behavior,
		such as not supporting implicit Git operations. Check relevant man pages
		for more information.

		If a flake ref is a path, it _MUST_ be absolute. Use *realpath(1)* if
		you must.

		Additionally, flake refs will usually be expanded when necessary.
		For example, the following flake ref:

			_github:water-sucks/nixed#CharlesWoodson_

		will get expanded to the following flake ref and attribute:

			_github:water-sucks/nixed#nixosConfigurations.CharlesWoodson_

		If the ref does not have text after the "#", then the NixOS system
		attribute name will be inferred to be the current system's hostname.

	*Legacy CLIs:*
		This can be a path to a configuration file directly
		(*configuration.nix*), or a directory containing a *default.nix*.

		Legacy configurations can also be sourced from the *$NIX_PATH*
		environment variable, if the _nixos-config=<PATH>_ attribute is
		specified there.

*NIXOS_GENERATION_TAG*
	A description for the generation. Usually set by default through the *-t*
	flag in *nixos apply* for child commands.

*NIXOS_CLI_DISABLE_STEPS*
	Disable showing visual steps with the logger. These "steps" get converted to
	information logs internally if this is set.

	Mostly useful for internal implementation, rather than for end-users.

*NIXOS_CLI_DEBUG_MODE*
	Show log messages for when developing TUIs. Only useful for during
	development.

*NIXOS_CLI_SUPPRESS_NO_SETTINGS_WARNING*
	Suppress the settings warning. Useful for non-NixOS systems where there is
	no settings file configured by default and the warnings get noisy/clutter
	logs.

# SEE ALSO

*nixos-cli-config(5)*

# AUTHORS

Maintained by the *nixos-cli* team. See the main man page *nixos-cli(1)* for
details.
