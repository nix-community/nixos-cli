NIXOS-CLI-REPL(1)

# NAME

nixos repl - start a Nix REPL with the current system's configuration loaded

# SYNOPSIS

nixos repl [FLAKE-REF] [options]

# DESCRIPTION

Launches an interactive Nix expression evaluator (REPL) preloaded with the
NixOS system configuration.

This command is useful for inspecting the configuration programmatically,
querying attributes, or testing expressions in the context of the active system.

If a flake-enabled CLI is in use, a flake ref may be supplied to specify the
configuration to load; otherwise, *$NIXOS_CONFIG* will be used and _must_
contain a valid flake ref.

If not, the environment variable *$NIXOS_CONFIG* will be
used, or the configuration can be passed through setting the *$NIX_PATH*'s
_nixos-config_ attribute properly through *-I* or elsewhere, or passed using
*FILE* and *ATTR* directly if already instantiated there.

# OPTIONS

*-h*, *--help*
	Show the help message for this command.

*-I*, *--include*
	Specify an additional location to search for Nix expressions. This behaves
	like passing *-I* to *nix repl* directly.

	It can be used to add custom search paths, such as
	*-I nixpkgs=/path/to/nixpkgs*.

# ARGUMENTS

*FLAKE-REF*  
	Optional flake reference to load attributes from. Only available on
	flake-enabled CLIs.

	Default: *$NIXOS_CONFIG*

*FILE*
	Specify an explicit path to a Nix file containing a NixOS configuration to
	load into the REPL.

	Only available on legacy CLIs.

	If not specified, then the value in the environment variable _$NIXOS_CONFIG_
	or the value of the _nixos-config_ entry in _$NIX_PATH_ will be used.

*ATTR*
	Use the specified attribute path inside of *FILE* as the NixOS system to
	use for the REPL.

	Only available on legacy CLIs.

	If *ATTR* is not specified, then the top-level attribute of *FILE* will be
	used.

# SEE ALSO

*nix repl(1)*

*nixos-cli-env(5)*

# AUTHORS

Maintained by the *nixos-cli* team. See the main man page *nixos-cli(1)* for
details.
