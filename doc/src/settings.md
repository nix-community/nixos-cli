# Settings

Settings are stored in `/etc/nixos-cli/config.toml`, and are stored in
[`TOML`](https://toml.io) format.

If preferred, this can be overridden by an environment variable
`NIXOS_CLI_CONFIG` at runtime. This is useful for testing configuration files.

Additionally, some configuration values can be overridden on the command-line
with the `--config` flag.

Example invocation:

```sh
$ nixos --config apply.imply_impure_with_tag=false apply
```

The preferred way to create this settings file is through the provided Nix
module that generates the TOML using the `programs.nixos-cli.config` option.
Refer to the [module documentation](./module.md) for other available options.

## Available Settings

These are the available settings for `nixos-cli` and their default values.

{{ #include generated-settings.md }}
