NIXOS-CLI-OPTION(1)

# NAME

nixos option - query available NixOS module options for this system

# SYNOPSIS

*nixos option* [options] [NAME]

# DESCRIPTION

Display documentation and values for available NixOS configuration options.
This command can be used to inspect how modules influence the system and what
values are currently set for individual options.

By default, *nixos option* will enter an interactive TUI with a search bar.
Non-interactive mode can be specified explicitly using *--non-interactive*/*-n*
or by using the alternative display options *--json* or *--value-only*. When in
non-interactive mode, a specific option name to display details for is required.

See *nixos-cli-option-ui(1)* for information on how the option TUI works.

If the option is found in non-interactive mode, then its details are displayed.
Otherwise, similar options are searched for, and printed if they roughly
match the search query.

# EXAMPLES

Find an option using the UI (starting with an initial search):

	*nixos option "search.for.option.with.this.name"*

Find an option and display its details, non-interactively:

	*nixos option -n \_module.args*

Find an option and obtain the type using structured JSON output and _jq_:

	*nixos option \_module.args -j | jq .type*

Find an option in a different flake ref (assume a flake-enabled CLI):

	*nixos option -n -f "github:MattRStoffel/mixed#nixos-machine" "option.name"*

# OPTIONS

*--attr* <PATH>
	Use the specified attribute path inside of *--file* as the NixOS system to
	evaluate options from.

	Only available on legacy CLIs.

	If *--attr* is not specified, then the top-level attribute of what is passed
	to *--file* will be used.

*--file* <PATH>
	Specify an explicit *PATH* to a Nix file containing a NixOS configuration
	to evaluate options from.

	Only available on legacy CLIs.

	This argument is useful for getting options for a NixOS system from a Nix
	file that is not a flake or a NixOS configuration module.

	If not specified, then the value in the environment variable _$NIXOS_CONFIG_
	or the value of the _nixos-config_ entry in _$NIX_PATH_ will be used.

*-f*, *--flake* <REF>
	Specify an explicit flake *REF* to evaluate options from. Only available
	on flake-enabled CLIs.

	If the cache is used to retrieve available options, some options that show
	up may actually not be available on the target configuration.

	Use the *--no-cache* flag to fully evaluate the option set for this
	configuration to avoid this issue.

	See *nixos-config-env(5)* for the proper flake ref format.

	Default: *$NIXOS_CONFIG*

*-j*, *--json*
	Output option data as a JSON object.

	Errors will have an "error" key along with "similar_options" with the
	list of at max 10 items that have been matched.

*--no-cache*
	Disable usage of the prebuilt options cache.

	Disabling the cache means that the index will need to be built, which takes
	time due to Nix evaluation being slow. Use only when the normal option cache
	is not working.

*-n*, *--non-interactive*
	Do not start the interactive search environment. This option is implied
	when *--json* or *--value-only* modes are in use.

*-s*, *--min-score* <SCORE>
	Minimum fuzzy match *SCORE* for filtering results. The bigger the number,
	the less search results will appear. However, the results will be more
	relevant as they appear if the score is higher.

	Default: *1*

*-v*, *--value-only*
	Print only the current value of the selected option.

	Useful for scripts where the option name is needed.

*-h*, *--help*
	Show the help message for this command.

# ARGUMENTS

*NAME*
	The name of the option to look up. If not provided, interactive mode
	is required to explore available options.

# SEE ALSO

*nixos-cli-option-ui(1)*

*nix-instantiate(1)*

*nix3-eval(1)*

# AUTHORS

Maintained by the *nixos-cli* team. See the main man page *nixos-cli(1)* for
details.
