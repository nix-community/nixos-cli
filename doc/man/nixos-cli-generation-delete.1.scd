NIXOS-CLI-GENERATION-DELETE(1)

# NAME

nixos generation delete - delete NixOS generations from this system

# SYNOPSIS

*nixos generation delete* [options] [GEN...]

# DESCRIPTION

Delete NixOS generations from this system based on a flexible set of
constraints.

You can specify individual generation numbers or combine options to tailor which
generations are deleted; the order of operations for these options is defined
below the options and arguments.

# EXAMPLES

Delete all generations older than 30 days but keep generation #42:

	*nixos generation delete --older-than '30d' --keep 42*

Delete generations from 42-69 (inclusive) and ensure that 5 generations remain:

	*nixos generation delete --from 42 --to 69 --min 5*

Delete all generations starting from 25 and ensure that 32 is always kept; also
delete 22 explicitly at the same time:

	*nixos generation delete 22 --from 25 --keep 25*

# OPTIONS

*-a*, *--all*
	Delete all generations except the current one.

*-f*, *--from* <GEN>
	Delete all generations after generation number *GEN*, inclusive.

	This will go all the way up to the latest generation number if not
	accompanied by the *--to* parameter, which sets an upper bound.

*-h*, *--help*
	Show the help message for this command.

*-k*, *--keep* <GEN>
	Always keep the specified generation number *GEN*. This option can be
	specified multiple times.

*-m*, *--min* <NUM>
	Ensure that a minimum of *NUM* generations _always_ exists.

*-o*, *--older-than* <DURATION>
	Delete all generations older than *DURATION*. The *DURATION* value is a
	*systemd.time(7)*-formatted time span, such as *"30d 2h 1m"*.

	For more information, see the *systemd.time(7)* man page.

*-p*, *--pattern* <REGEX>
	Delete all generations that have a tag/description match *REGEX*.

	The *REGEX* value is a Go-formatted regular expression using RE2
	syntax, which is roughly a subset of PCRE regular expression syntax.

	For more information, see https://github.com/google/re2/wiki/Syntax.

*-t*, *--to* <GEN>
	Delete all generations until generation number *GEN*, inclusive.

	This will go all the way down to the earliest generation number if not
	accompanied by the *--from* parameter, which sets a lower bound.

*-v*, *--verbose*
	Enable verbose logging.

*-y*, *--yes*
	Automatically confirm generation deletion without any interactive prompt.

# ARGUMENTS

*[GEN]...*
	One or more specific generation numbers to delete. These can be used
	alongside the options for more fine-grained control, but are optional.


Options and arguments can be combined ad-hoc to create complex filtering
constraints.

# ORDER OF OPERATIONS

The order of evaluating generations to delete vs. which ones to keep is this,
from most prioritized to least prioritized:

	- *--min*
	- *--keep*
	- *[GEN...]* positional args
	- *--all*
	- *--pattern*
	- *--from* + *--to*
	- *--older-than*

And for any range where the generation numbers to keep can be ambiguous, the
most recent generations will be kept.

# SEE ALSO

*nixos-cli-generation(1)*

*systemd.time(7)*

# AUTHORS

Maintained by the *nixos-cli* team. See the main man page *nixos-cli(1)* for
details.
