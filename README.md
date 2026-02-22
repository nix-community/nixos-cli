<h1 align="center">nixos-cli</h1>
<h6 align="center">A unified NixOS management tool.</h6>

## Introduction

Tooling for `nixos` has become quite scattered, and as a result, NixOS can be
pretty obtuse to use. There are many community tools available to fix the
problem, but no all-in-one solution.

`nixos-cli` is exactly that - an all-in-one tool to manage any NixOS
installation with ease, that consists of:

- Drop-in replacements for NixOS scripts and tools like `nixos-rebuild`
- Generation manager and option preview TUIs
- Many more

All available through an easy-to-use (and pretty!) interface.

High-level documentation is available as a
[website](https://nix-community.github.io/nixos-cli), while a detailed reference
for each command and settings is available in the form of man pages after
installation.

## Development

This application is written in [Go](https://go.dev).

There are two major directories to keep in mind:

- `cmd/` :: command structure, contains actual main command implementations
- `internal/` :: anything that is shared between commands, categorized by
  functionality

Each command and subcommand **MUST** go in their own package and match the
command tree that it implements.

All dependencies for this project are neatly provided in a Nix shell. Run
`nix develop .#` or use [`direnv`](https://direnv.net) to automatically drop
into this Nix shell on changing to this directory.

In order to build both packages at the same time, run
`nix build .#{nixos-cli,nixos-cli-legacy}`.

Legacy-style `nix-build` and `nix-shell` can also be used; this uses
`flake-compat` under the hood.

### Tests

A collection of NixOS integration tests, created using
`pkgs.testers.runNixOSTest` is available in [nix/tests](./nix/tests). They are
also exposed as flake check attributes.

To run a particular test using flakes, run
`nix build .#checks.<arch>.<test-name>`.

If not using the flake, invoke `nix-build` directly using
`nix-build -A <test-name>` to run a test.

Large features with a significant feature scope should likely have corresponding
NixOS tests for them, so create more as needed. When creating a test, use a
suffix `.test.nix` and place it in `./nix/tests`, and it will be picked up as a
check automatically based on the filename.

Since these tests are quite expensive to run, passing them is not an automatic
CI requirement, but care should be taken to not fail the tests in this
directory, and pull requests will be rejected if they are not fixed.

### Documentation

Documentation is split into two parts:

- A documentation website, built using
  [`mdbook`](https://rust-lang.github.io/mdBook/)
- Manual pages (`man` pages), generated using
  [`scdoc`](https://sr.ht/~sircmpwn/scdoc/)

They are both managed with a build script at [doc/build.go](./doc/build.go), and
with the following Makefile rules:

- `make gen-manpages` :: generate `roff`-formatted man pages with `scdoc`
- `make gen-site` :: automatically generate settings/module docs for website
- `make serve-site` :: start a preview server for the `mdbook` website.

`make gen-site` generates two files:

- Documentation for all available settings in `config.toml`
- Module documentation for `programs.nixos-cli`, built using
  [`optnix`](https://github.com/water-sucks/optnix)

The rest of the site documentation files are located in [doc/man](./doc/src).

`make gen-manpages` generates man pages using `scdoc`, and generates one
additional man page file from a template: the available settings for
`nixos-cli-config(5)`.

Check the build script source for more information on how to work with this.

### Versioning

Version numbers are handled using [semantic versioning](https://semver.org/).
They are also managed using Git tags; every version has a Git tag named with the
version; the tag name does not have a preceding "v".

Non-released builds have a version number that is suffixed with `"-dev"`. As
such, a tag should always exist on a version number change (which removes the
suffix), and the very next commit will re-introduce the suffix.

Once a tag is created and pushed, create a GitHub release off this tag.

The version number is managed inside the Nix derivation at
[package.nix](./nix/package.nix).

### CI

The application must build successfully upon every push to `main`, and this is a
prerequisite for every patch or pull request to be merged.

Cache artifacts are published in a Cachix cache at https://watersucks.cachix.org
when a release is triggered.

## Talk!

Join the Matrix room at
[#nixos-cli:matrix.org](https://matrix.to/#/#nixos-cli:matrix.org)! It's open
for chatting about NixOS in general, and for making it a better experience for
all that involved.

I would like for this to become a standard NixOS tool, which means that I want
to cater to potentially many interests. If you would like for any commands to be
implemented that you think fit this project, talk to me on Matrix or file a
GitHub issue.
