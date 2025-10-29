NIXOS-CLI-APPLY(1)

# NAME

nixos apply - build and/or activate a NixOS configuration

# SYNOPSIS

*nixos apply* [FLAKE-REF] [options]

# DESCRIPTION

Build and activate a NixOS system configuration.

This command performs the equivalent of evaluating the NixOS module system,
building the system configuration, and activating the resulting derivation.
It may also install the bootloader and manage boot entries, depending on
provided options.

This command behaves slightly differently based on if it is flake-enabled
or not, and this requirement will be specified for the relevant options
and sections.

# EXAMPLES

Many behaviors are similar to the _nixos-rebuild_ command of old. Here are some
common commands, alongside their _nixos-rebuild_ equivalents.

_`nixos-rebuild switch`_

	*nixos apply*

_`nixos-rebuild switch`_, without interactive confirmation

	*nixos apply -y*

_nixos-rebuild switch_, on an arbitrary flake ref (for flake-enabled CLIs only)

	*nixos apply "github:water-sucks/nixed#CharlesWoodson"*

_nixos-rebuild test_

	*nixos apply --no-boot*

_`nixos-rebuild vm[-with-bootloader]`_

	*nixos apply --vm[-with-bootloader] --output ./vm*

_nixos-rebuild boot_

	*nixos apply --no-activate*

_nixos-rebuild dry-activate_

	*nixos apply --dry*

_nixos-rebuild dry-build_

	*nixos apply --dry --no-activate --no-boot --output ./result*

Many other behaviors for _nixos-rebuild_ are in the *nixos generation*
command tree. See *nixos-cli-generation(1)* for details.

## Rollback

In rare cases, automatic rollback is performed on the system profile when
activation fails.

In order to disable this behavior for known working configurations (such as when
reboot is required for a successful activation or daemon restart), use the
global *--config* flag as such:

	*nixos --config auto_rollback=false apply [options]*

# ARGUMENTS

*FLAKE-REF*
	Specify an explicit flake ref with a NixOS configuration. Only available
	on flake-enabled CLIs.

	See *nixos-config-env(5)* for the proper flake ref format.

	Default: *$NIXOS_CONFIG*

# OPTIONS

*--build-host* <HOST>
	Instead of building the new configuration locally, use the specified *HOST*
	to perform the build. The host needs to be accessible with SSH, and must be
	able to perform Nix builds.

	*HOST* is a simple SSH address with the format "_user@address:port_".

	SSH keys must be loaded into _ssh-agent_ for them to be used by the program.
	If an agent is not detected, then an error will be displayed.

	If *--target-host* is not set, the build will be copied back to the local
	machine when done.

*-d*, *--dry*
	Perform a dry run. Show what would be built or executed without making
	changes.

	Cannot be specified alongside *--output*.
	
	There are two modes of dry-ness: dry activation and dry builds.

	Dry builds are Nix builds that show what will be built. On the other hand,
	dry activations show what will be ran in the activation script, and require
	a real configuration to be built beforehand.

	Dry builds of the configuration can be achieved by passing *--no-activate*
	and *--no-boot*, as well as *--output*. If any of these options are not
	passed, then dry activation is assumed.

	The list of changes is not guaranteed to be complete for dry activations.

*--install-bootloader*
	(Re)install the bootloader to the configured target device(s). The
	bootloader configuration is managed by their NixOS modules, so this behavior
	will vary depending on the relevant NixOS configuration options.

*--no-activate*
	Do not run activation scripts. Useful for setting up a system to boot later,
	or for build-only runs.

*--no-boot*
	Skip registering the generation as a boot entry. Useful for testing a NixOS
	system configuration, or for build-only runs.

*-o*, *--output* <PATH>
	Symlink the result of the build to the given *PATH*.

*-p*, *--profile-name* <NAME>
	Specify the Nix profile *NAME* used for tracking generations.

	Default: *system*

*--remote-root*
	Prefix remote activation commands with an escalation command like _sudo_.

	Only applies for remote hosts.

	This is applicable to switching the active Nix profile and for running
	the _switch-to-configuration_ script, so users may need to type in a
	password twice if NOPASSWD is not set for that user.

	This is defined by the setting _root_command_.

*-s*, *--specialisation* <NAME>
	Activate a specialisation *NAME* from the available specialisations.

	If none are provided, then the base configuration without specialisations
	is assumed unless there is a default one specified in
	*apply.specialisation*. Check *nixos-cli-settings(5)* for more information.

	This requires activation, and as such, conflicts with the *--no-activate*
	option.

*-t*, *--tag* <DESCRIPTION>
	Tag this generation with the given *DESCRIPTION*.

	Useful for identifying its purpose later.

	This option may require *--impure* to be passed for flake configurations,
	unless *apply.imply_impure_with_tag* is set to _true_ in the settings. This
	is due to the internal implementation that uses an environment variable to
	pass it to the Nix configuration.

	Additionally, a tag may be inferred from Git commit messages if the
	option *apply.use_git_commit_msg* is enabled. This only works for local
	paths, and will not work with remote flake refs.

*--target-host* <HOST>
	Deploy the built configuration to the specified *HOST* and activate it. The
	host needs to be accessible with SSH. For activation-related code to run,
	root access is required.

	If this is left empty, activation of the system will happen locally.

	*HOST* is a simple SSH address with the format "_user@address:port_".

	If *--build-host* is not explicitly specified or is left empty, building
	will take place locally.

	Note that this program honors the _nixpkgs.crossSystem_ setting of the given
	configuration but disregards the true architecture of the target host.
	Hence, the nixpkgs.crossSystem setting has to match the target platform, or
	else activation will fail.

*--upgrade*
	Upgrade the root user's _nixos_ Nix channel before building the
	configuration, as well as any Nix channels with a file named
	_.update-on-nixos-rebuild_ in it.

	This option only exists for legacy CLIs, rather than flake-enabled ones.

*--upgrade-all*
	Like *--upgrade*, but for all of the root user's Nix channels.

	This option only exists for legacy CLIs, rather than flake-enabled ones.

*--use-nom*
	Use *nix-output-monitor* (_nom_) to display build progress.

	Requires that _nom_ is installed and available in the *$PATH*.

*-v*, *--verbose*
	Enable verbose logging during the operation.

*--vm*
	Instead of activating the configuration, build a NixOS VM boot script.

*--vm-with-bootloader*
	Like *--vm*, but includes the bootloader setup in the generated VM script.

	Only set one or the other, not both.

*-y*, *--yes*
	Automatically confirm activation steps, skipping interactive prompts.

*-h*, *--help*
	Show the help message for this command.

# NIX OPTIONS

*nixos apply* accepts some Nix options and passes them through to their relevant
Nix invocations.

The following options are supported:

- *--quiet*
- *--print-build-logs*, *-L*
- *--no-build-output*, *-Q*
- *--show-trace*
- *--keep-going*, *-k*
- *--keep-failed*, *-K*
- *--fallback*
- *--refresh*
- *--repair*
- *--impure*
- *--offline*
- *--no-net*
- *--substitute-on-destination*, *--use-substitutes*
- *--max-jobs*, *-j*
- *--cores*
- *--builders*
- *--log-format*
- *--include*, *-I*
- *--option* <KEY=VALUE> (single argument, separated by an = sign)

*--option* is specified slightly differently; for *nixos-cli* to pass it through
properly, pass the option key and value as a single argument, rather than as two
separate arguments in the actual Nix CLI.

The following options are supported on flake-enabled CLIs:

- *--recreate-lock-file*
- *--no-update-lock-file*
- *--no-registries*, *--no-use-registries*
- *--commit-lock-file*
- *--update-input*
- *--override-input* <INPUT=VALUE>

*--override-input* is specified slightly differently; for *nixos-cli* to pass it
through properly, pass the input name and value as a single argument separated
by an equals sign, rather than as two arguments in the actual Nix CLI.

# SEE ALSO

*nixos-cli-generation(1)*

*nixos-cli-option(1)*

*nix3-build*(1), *nix-build(1)*

*nix-env(1)*

*nixos-cli-env*(5)

# AUTHORS

Maintained by the *nixos-cli* team. See the main man page *nixos-cli(1)* for
details.
