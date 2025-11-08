NIXOS-CLI-INSTALL(1)

# NAME

nixos install - install a NixOS system from a provided configuration

# SYNOPSIS

Flake-enabled CLIs:

*nixos install* [FLAKE-REF] [options]

Legacy CLIs:

*nixos install* [FILE] [ATTR] [options]

# DESCRIPTION

*nixos install* installs a NixOS system by:

	- Initialize the Nix store and copy an initial _nixos_ channel
	- Building and creating an initial generation on the target mountpoint
	- Installing a bootloader if configured
	- Setting a root password

The installed configuration will depend on if the CLI is flake-enabled or not.

If the CLI is flake-enabled:
	- The *FLAKE-REF* argument is _required_, and must point to a valid
	  flake reference with a NixOS configuration.
Otherwise:
	- The *$NIXOS_CONFIG* variable must point to a valid NixOS configuration
	  module or a directory containing a _default.nix_ with the same.
	- OR the target root must have a file at _/etc/nixos/configuration.nix_
	- OR *FILE* + *ATTR* should be passed.

The installation will take place relative to a specified root directory
(defaults to `/mnt`). Mountpoints and all other filesystems must be mounted
in the correct place prior to installation, or they will not exist on the
target system.

*nixos install* is idempotent, and as such, can be used to re-install broken
NixOS systems without needing to wipe filesystems first, in most cases. That
means this command can be re-run if problems arise during installation, and
can also be ran on already-existing NixOS installations.

In the future, remote installations will be supported. Look to the release page
on GitHub for when this feature comes out.

# EXMAPLES

In order to create a fresh NixOS installation, mountpoints need to be set up
beforehand.

This block of commands will show a typical installation procedure, assuming the
following simple partition layout on a UEFI system:

	/dev/sda1 :: EFI System Partition (512M, FAT32)
	/dev/sda2 :: NixOS root partition (ext4)

Typically, a user will also need to obtain *nixos-cli* in a Nix shell, as it is
not available on traditional NixOS install ISOs or Linux distributions, as of
yet.

Refer to the installation steps in the online manual to obtain the *nixos-cli*
binary in a development shell.

Installation steps:

	_$ mkfs.fat -F 32 /dev/sda1_

	_$ mkfs.ext4 /dev/sda2_

	_$ mount /dev/sda2 /mnt_

	_$ mkdir -p /mnt/boot_

	_$ mount /dev/sda1 /mnt/boot_

	_$ nixos init --root /mnt_

	# Change whatever you will need to in the configuration.

	# For flake-enabled CLIs, use the following invocation:

	_$ nixos install --root /mnt --flake /mnt/etc/nixos#<SYSTEM>_

	# For non-flake CLIs, use the following invocation:

	_$ nixos install --root /mnt_

	# Reboot

Running *nixos enter --root /mnt* in order to inspect the new installation
is also possible, assuming a successful installation.

# ARGUMENTS

*FLAKE-REF*
	Specify an explicit flake ref to evaluate options from. Only available
	on flake-enabled CLIs.

	See *nixos-config-env(5)* for the proper flake ref format.

	The system name is NOT inferred from the hostname, and must be provided
	explicitly after the #.

	There is no fallback, so if this argument is not provided on a flake-enabled
	CLI, the program will fail.

*FILE*
	Specify an explicit path to a Nix file containing a NixOS configuration.

	Only available on legacy CLIs.

	This argument is useful for building a NixOS system from a Nix file that is
	not a flake or a NixOS configuration module.

	If not specified, then the value in the environment variable _$NIXOS_CONFIG_
	or the value of the _nixos-config_ entry in _$NIX_PATH_ will be used.

*ATTR*
	Use the specified attribute path inside of *FILE* as the NixOS system to
	build.

	Only available on legacy CLIs.

	If *ATTR* is not specified, then the top-level attribute of *FILE* will be
	used.

# OPTIONS

*-c*, *--channel* <PATH>
	Use the derivation at *PATH* as the _nixos_ channel to copy to the target
	system.

	If not provided, then the existing _nixos_ channel for the root user on the
	running system will be copied instead. This is usually the case on live
	NixOS USBs or external media used for fresh installations.

*-h*, *--help*
	Show the help message for this command.

*--no-bootloader*
	Do not install the bootloader on the target device.

	For fresh installations, it is recommended not to enable this option.
	Proceed at your own risk.

*--no-channel-copy*
	Do not copy a NixOS channel to the target system.

	This is useful for speeding up installations if the target NixOS channel
	already exists, or if using flake configurations that do not require
	Nix channels configured at all.

	Conflicts with *--channel*.

*--no-root-passwd*
	Skip prompting to set the root password.

	Useful for non-interactive installation, such as in scripts.

*-r*, *--root* <DIR>
	Treat *DIR* as the root directory for installation.

	Default: */mnt*

*-s*, *--system* <PATH>
	Install the system from an already built system closure at *PATH*.

	This is useful for installing multiple systems from the same system closure,
	to avoid repeated _nix build_ calls.

*-v*, *--verbose*
	Enable verbose logging.

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
- *--max-jobs*, *-j*
- *--cores*
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
through properly, pass the input name and value as a single argument, rather
than as two arguments in the actual Nix CLI.

# SEE ALSO

*nixos-cli-enter(1)*

*nixos-cli-init(1)*

*nix3-build*(1), *nix-build(1)*

*nix-env(1)*

*nixos-cli-env*(5)

# AUTHORS

Maintained by the *nixos-cli* team. See the main man page *nixos-cli(1)* for
details.
