# Installation

`nixos-cli` is split into two separate executables. This is very intentional for
a number of reasons.

1. The majority of NixOS users use either flakes or legacy-style Nix, without
   mixing the two.
2. While the majority of logic is shared between the two styles of
   configuration, the command-line interface should not be forced to deal with
   the differences, for the sake of clarity.
3. If users want to mix styles, they should do so intentionally. This
   distinction is reflected in the CLI binaries themselves—not hidden in command
   behavior.

The flake-style configuration is the default. Nix flakes have been available for
several years; although still technically experimental, they are widely adopted
and considered stable in practice, particularly in forks like
[Lix](https://lix.systems). Legacy configurations are actively supported
regardless of this status, though.

NixOS has quite a large ecosystem of tools, and can be quite the moving target
in terms of features, so `nixos-unstable` and the current stable release are the
only actively supported releases.

## Adding To Configuration

Use the following sections depending on whether or not your systems are
configured with flakes or legacy-style configurations.

Available configuration settings for `nixos-cli` are defined in the more
detailed [settings](./usage/settings.md) section, and are specified in Nix
attribute set format here. Internally, they are converted to TOML.

### Flakes

`nixos-cli` is provided as a flake input. Add this and the exported NixOS module
to the system configuration.

```nix
{
  inputs.nixos-cli.url = "github:nix-community/nixos-cli";

  outputs = { nixpkgs, nixos-cli, ... }: {
    nixosConfigurations.system-name = nixpkgs.lib.nixosSystem {
      modules = [
        nixos-cli.nixosModules.nixos-cli
        # ...
      ];
    };
  };
}
```

Then, enable the module.

```nix
{ config, pkgs, ... }:

{
  programs.nixos-cli = {
    enable = true;
    config = {
      # Whatever settings desired.
    }
  };
}
```

The default package is flake-enabled in this setup, so the
`programs.nixos-cli.package` option does not need to be specified.

### Legacy

To use the NixOS module in legacy mode, import the `default.nix` provided in
this repository. An example is provided below with `builtins.fetchTarball`:

```nix
{ config, system, pkgs, ...}:

let
  # In pure evaluation mode, always use a full Git commit hash instead of a branch name.
  nixos-cli-url = "github:nix-community/nixos-cli/archive/GITREVORBRANCHDEADBEEFDEADBEEF0000.tar.gz";
  nixos-cli = import "${builtins.fetchTarball nixos-cli-url}" {inherit pkgs;};
in {
  imports = [
    nixos-cli.module
  ];

  programs.nixos-cli = {
    enable = true;
    config = {
      # Other configuration for nixos-cli
    };
  };

  # ... rest of config
}
```

NOTE: By default, importing like this will use the `nixosLegacy` package by
default, so there is no need to specify the `programs.nixos-cli.package`
attribute manually in this setup unless overriding something.

## Cache

There is a Cachix cache available. Add the following to your NixOS configuration
to avoid lengthy rebuilds and fetching extra build-time dependencies:

```nix
{
  nix.settings = {
    substituters = [ "https://watersucks.cachix.org" ];
    trusted-public-keys = [
      "watersucks.cachix.org-1:6gadPC5R8iLWQ3EUtfu3GFrVY7X6I4Fwz/ihW25Jbv8="
    ];
  };
}
```

Or if using the Cachix CLI outside a NixOS environment:

```sh
$ cachix use watersucks
```

There are rare cases in which you want to automatically configure a cache when
using flakes, such as when installing NixOS configurations using this tool. The
following configuration in the `flake.nix` can help with this:

```nix
{
  nixConfig = {
    extra-substituters = [ "https://watersucks.cachix.org" ];
    extra-trusted-public-keys = [
      "watersucks.cachix.org-1:6gadPC5R8iLWQ3EUtfu3GFrVY7X6I4Fwz/ihW25Jbv8="
    ];
  };

  inputs = {}; # Whatever you normally have here
  outputs = inputs: {}; # Whatever you normally have here
}
```

⚠️ Beware, though: this is a relatively undocumented feature—use with caution.

## Running Using Nix Shells

Sometimes, you may not want to add it to your configuration, and instead run
`nixos-cli` on an ad-hoc basis.

This is the preferred way to use `nixos-cli` when running `nixos init` or
`nixos install` on a live NixOS USB for installation.

Use `nix develop` (flake-enabled package by default):

```
$ nix shell github:nix-community/nixos-cli
```

Alternative using legacy-style `nix-shell` and the `nixosLegacy` package:

```sh
$ nix-shell -E 'with import (fetchTarball "https://github.com/nix-community/nixos-cli/archive/refs/heads/main.tar.gz") {}; nixosLegacy'
```

## Rebuild

After adding the next sections to your configuration, rebuild your configuration
once, and then the `nixos` command should be available. Verify by running
`nixos features`:

```sh
# Example output of `nixos features`
$ nixos features
nixos 0.13.0-dev
git rev: 53beba5f09042ab8361708a5e0196098d642ba5b
go version: go1.24.1
nix version: nix (Nix) 2.28.2

Compilation Options
-------------------
flake           :: true
nixpkgs_version :: 24.11
```

Nice! `nixos-cli` is now ready for usage.
