# Overview

## NixOS Tooling Replacements

`nixos-cli` has drop-in replacements for the following tools:

- `nixos-rebuild` → `nixos apply` + `nixos generation`
- `nixos-enter` → `nixos enter`
- `nixos-generate-config` → `nixos init`
- `nixos-version` → `nixos info`
- `nixos-install` → `nixos install`
- `nixos-info` → `nixos manual`

### `nixos apply` + `nixos generation`

`nixos-rebuild` is primarily used to manage NixOS configurations, but has become
bloated and has some non-obvious behavior.

`nixos-rebuild` commands are replaced entirely by a combination of the
`nixos apply` and `nixos generation` commands, with some better-looking logging,
showing diffs between generations, and interactive confirmation before applying
configurations.

Alternatives to builtin Nix tools are provided that can be switched through the
settings if available; for example:

- [`nix-output-monitor`](https://github.com/maralorn/nix-output-monitor) for
  building configurations
- [`nvd`](https://khumba.net/projects/nvd/) for showing generation diffs

A list of analogues to `nixos-rebuild` behavior:

```sh
# `nixos-rebuild switch`
$ nixos apply

# `nixos-rebuild switch`, without interactive confirmation
$ nixos apply -y

# `nixos-rebuild switch` on an arbitrary flake ref
$ nixos apply "github:water-sucks/nixed#CharlesWoodson"

# `nixos-rebuild test`
$ nixos apply --no-boot

# `nixos-rebuild vm[-with-bootloader]`
$ nixos apply --vm[-with-bootloader] --output ./vm

# `nixos-rebuild boot`
$ nixos apply --no-activate

# `nixos-rebuild list-generations`
$ nixos generation list

# Show diffs between two generation numbers on the local system
$ nixos generation diff 59 60

# Switch to an arbitrary generation number (and specialisation)
$ nixos generation switch 420 [--specialisation "wayland"]

# `nixos-rebuild switch --rollback`
$ nixos generation rollback

# Fine-tuned generation deletion; keep at least five generations, delete the rest
$ nixos generation delete --min 5 --all
```

Check the manual for more important information.

Setting the `$NIXOS_CONFIG` variable allows for not specifying the `--flake`
flag at _all_, which is a huge improvement over `nixos-rebuild`.

`nixos generation list` by default is a TUI list with Vim-like bindings. To get
tabular, `grep`-able output like the old behavior of `nixos-rebuild` uses, use
`-t`.

Default specialisations are managed through the `nixos-cli` configuration.

In the future, setting build and target hosts for remote building/activation
using SSH will be supported.

### `nixos-enter`

`nixos enter` behaves mostly the same as `nixos-enter`, minus some extra logging
controls.

### `nixos-generate-config` -> `nixos init`

`nixos init` can be used in the same way as `nixos-generate-config`. Usually,
this is done through a NixOS live USB before installation. As such, refer to the
[installation](./installation.md) section for instructions on how to do that.

**NOTE**: The current configuration that is generated does not include
`nixos-cli` setup, due to implementation complexity. If you believe this is
important enough, please file a feature request.

### `nixos-install` -> `nixos install`

`nixos install` can also be used in the same way as the current `nixos-install`
script. Similar to `nixos init` usage, this is likely to be done off a live USB,
rather than on a live system.

In the future, remote NixOS installations will be supported.

### `nixos features`

This command describes the features that `nixos-cli` was compiled with.

Use this when filing issues, in order to provide information about the
environment for proper problem diagnosis.

## Option UI

The option UI is a nice search for NixOS options that are available on a given
system. These are computed on demand for the system, so _all_ available options
on that exact system are present.

This is a significant advantage over alternatives; since options are computed
from the modules present in a given system, modules that don't have module
documentation exposed can _still_ have documentation through the option UI!

Cool, right?

By default, `nixos option` uses this TUI.

To use normal, non-interactive output for a specific option, add the `-n`
switch.

However, there is one caveat: generating the option index is an intensive
operation; this can be precomputed on every configuration change using the
`services.nixos-cli.prebuildOptionCache` if desired.

## Environment Variables

The following environment variables influence `nixos-cli` behavior:

- `NO_COLOR` :: disable output color (does not apply for TUIs)
- `NIXOS_CLI_CONFIG` :: change the `nixos-cli` settings location (default:
  `/etc/nixos-cli/config.toml`)
- `NIXOS_CONFIG` :: where the configuration to work with is stored

  This can vary depending on if the CLI is flake-enabled. If the CLI is
  flake-enabled, then `$NIXOS_CONFIG` _must_ point to a valid flake ref.
  Otherwise, it can point to a local Nix configuration file (i.e.
  `configuration.nix`) or directory containing a `default.nix`.

## Aliases

Aliases can be used to make shortcuts for `nixos-cli` commands. Check the
[settings](./settings.md) section for an example.

A list of common aliases that people can configure:

- `nixos genlist` :: `nixos generation list`
- `nixos switch` :: `nixos generation switch`
- `nixos testcfg` :: `nixos apply --no-boot --no-activate`
- `nixos build` :: `nixos apply --no-boot --no-activate --output ./result`

**NOTE**: Currently, aliases to compose multiple CLI commands or to invoke shell
commands are not supported. If this is important to you, please file a feature
request.

## Completion

Shell completion is provided through the default package. Descriptions for
completion candidates are provided (requires newer Bash versions if applicable).

If desired, completion scripts can be obtained manually using
`nixos completion <SHELL>`.

Supported shells:

- `bash`
- `zsh`
- `fish`

If you want support for another shell, file an issue. However, this support is
provided by [Cobra](https://github.com/spf13/cobra), and as such, support will
probably need to be implemented upstream before this is possible.
