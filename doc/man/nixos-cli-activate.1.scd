NIXOS-CLI-ACTIVATE(1)

# NAME

nixos activate - run activation routines for a NixOS system

# SYNOPSIS

*nixos activate* <ACTION> [options]

# DESCRIPTION

Run activation routines for a given NixOS system.

This command handles migrating (switching) between built NixOS configurations,
built from a NixOS configuration's _config.system.build.toplevel_ build output.

If ran directly by a user, the switch script, located at
_/run/current-system/bin/switch-to-configuration_, will be executed. If using
the activation interface NixOS module, this calls back out to _nixos activate_
with the correct parameters for the given generation.

This activation script is mostly backwards-compatible with the existing
_switch-to-configuration-ng_ script that is provided in upstream _nixpkgs_.

This is a low-level command, and should be regarded accordingly.

# ARGUMENTS

*ACTION*
	The activation routine to run.

	Can be exactly one of the following values:

	- *boot* - Generate boot entries and make this the default configuration
	- *check* - Run pre-activation checks and exit
	- *dry-activate* - Show what would be activated but do not perform it
	- *switch* - Activate this configuration, generate entries, and make this
	  the default configuration
	- *test* - Activate this configuration, but do not generate boot entries

# OPTIONS

*-s*, *--specialisation* <NAME>
	Activate a specialisation *NAME* from the available specialisations.

	If none are provided, then the base configuration without specialisations
	is assumed. Check *nixos-cli-settings(5)* for more information.

	This is only available in user-direct invocation mode, to figure out which
	specialisation to use, and does not exist during actual activation mode.

*-h*, *--help*
	Show the help message for this command.

# SEE ALSO

*nixos-cli-apply(1)*

*nixos-cli-env*(5)

# AUTHORS

Maintained by the *nixos-cli* team. See the main man page *nixos-cli(1)* for
details.
