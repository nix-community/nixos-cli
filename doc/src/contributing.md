# Contributing

Contributions of all kinds are appreciated! The following are especially
welcome.

## Code Contribution

This is _by far_ the best way to help!

`nixos-cli` is quite a large tool with a very large scope and many moving parts,
so any efforts to ease the burden around implementation and maintenance is
greatly appreciated. Even so-called "drive-by" contributions or features are
also appreciated, as long as they do not result in excessive maintenance burden
later on.

Submit contributions through pull requests, or by emailing them personally to
`varun@snare.dev`, if you do not want to use GitHub. Credit will be preserved.

Please make sure your code is up to standard by running:

- `gofmt` to format Go code
- `golangci-lint` to catch common issues
- `prettier` to format Markdown files

All available dependencies are provided in a Nix development shell.

If your changes modify the CLI or any core behavior, please also update the
relevant `man` pages or documentation in `doc/`. This includes changes to the
following things:

- CLI commands/options
- Settings
- NixOS module options

## Bug Reports

Testing every feature edge-case is hard—especially before full releases.

If you're a brave soul, use the main branch instead of a release version, and
file bug reports by
[opening a new issue](https://github.com/nix-community/nixos-cli/issues) with
the **Bug Report** template. In the bug report, provide:

- A clear description of the problem
  - **IMPORTANT**: What was _expected_ vs. what actually _happened_
- Steps to reproduce the issue
- Your environment (run `nixos features`)
- Any relevant logs, error messages, or images

Clear reports will assist in faster bug fixes!

## Improving Documentation

Nix documentation is notoriously patchy — so help here is _especially_ welcome.

As such, documentation quality is of utmost importance. `nixos-cli` should be a
tool that is both easy to use and powerful in functionality; however, as
powerful as it can be, who cares if that power isn't discoverable?

Documentation lives in two places:

- Markdown files for this website, generated using
  [`mdbook`](https://rust-lang.github.io/mdBook/)
- Manual pages (`man` pages), generated using
  [`scdoc`](https://sr.ht/~sircmpwn/scdoc/)

Refer to the code contribution guidelines when submitting documentation
improvements, or file an issue if the documentation issues are substantial.

## Feature Suggestions

Have an idea for improving NixOS tooling here? Start a discussion or open an
issue!

Discourse around how this can be done is always productive.

The vision is to make this a standard NixOS tool, so all ideas that align with
that scope are welcome. If there’s a new command or sub-tool you’d like to see,
open a GitHub issue or reach out on Matrix. However, try to keep it within scope
of the NixOS project, though.

❌ Features like `home-manager` or `nix-darwin` integration will not be
considered as first-class features. Sorry in advance.

## Community Conduct

All contributors must follow a friendly, respectful code of conduct.

The TL;DR? **Don't be a dick.**

Disagreement is fine, but harassment, rudeness, or discrimination are not
tolerated in any spaces.
