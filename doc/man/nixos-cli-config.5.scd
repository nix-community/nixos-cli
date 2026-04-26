NIXOS-CLI-CONFIG(5)

# NAME

nixos-cli-config - how NixOS configurations work in _nixos-cli_

# DESCRIPTION

*nixos-cli* primarily works with NixOS configurations.

A *configuration* in this document is defined as the entrypoint from
which a NixOS system is built.

This can take three forms:

- *Flake references* (abbreviated as flake "refs")
- *Explicit* configurations (Nix files that contain NixOS configurations)
- *Implicit* configurations (A Nix module that gets imported by 
  *<nixpkgs/nixos>*)

Flake-enabled CLIs only support flake references.

Legacy CLIs support both explicit and implicit configurations, and will
prioritize explicit ones over implicit ones.

## FLAKE REFERENCES

Note: these are only supported in flake-enabled CLIs.

Flake refs have a two-part format: *<URI>*#*<SYSTEM>*.

The *URI* can be any valid Nix filesystem tree containing a _flake.nix_ file in
the root. More information on the URI format can be found in *nix3-flake(1)*.

The *SYSTEM* portion corresponds to an attribute under the _nixosConfigurations_
output in the flake schema. They will usually be expanded when necessary. For
example, the following NixOS configuration flake ref:

	_github:water-sucks/nixed#CharlesWoodson_

will get expanded to the following flake ref and attribute:

	_github:water-sucks/nixed#nixosConfigurations.CharlesWoodson_

If the ref does not have text after the "#", then the NixOS system attribute
name will be inferred to be the current system's hostname unless otherwise
specified.

Flake refs without a local path may have slightly different behavior, such as
not supporting implicit Git operations. Check relevant man pages for more
information.

## EXPLICIT CONFIGURATIONS

Note: these are only supported in legacy CLIs.

Explicit configurations are self-contained Nix files that can be built
without relying on _$NIX_PATH_ or Nix channels.

Example:

```
let
  nixpkgs = fetchTarball {
    name = "nixos-unstable-2026-04-20";
    url = "https://github.com/NixOS/nixpkgs/archive/b12141ef619e0a9c1c84dc8c684040326f27cdcc.zip";
    sha256 = "0vhprxh6zqrc8bc745crfzs75cl1sqls3hdldlairm0spqsb88k5";
  };
in
  import "${nixpkgs}/nixos" {
    configuration = ./configuration.nix;
  }
```

_configuration.nix_ would be the entrypoint for the NixOS module that gets
imported.

Explicit configurations come in two forms; they can either contain a single
toplevel attribute that contains a buildable NixOS system, or they can contain
an attribute set of many NixOS systems.

Example:

```
let
  nixpkgs = fetchTarball {
    name = "nixos-unstable-2026-04-20";
    url = "https://github.com/NixOS/nixpkgs/archive/b12141ef619e0a9c1c84dc8c684040326f27cdcc.zip";
    sha256 = "0vhprxh6zqrc8bc745crfzs75cl1sqls3hdldlairm0spqsb88k5";
  };

  nixosSystem = configuration: import "${nixpkgs}/nixos" {
    inherit configuration;
  };
in {
  BoJackson = nixosSystem ./systems/BoJackson.nix;
  CharlesWoodson = nixosSystem ./systems/CharlesWoodson.nix;
  SebastianJanikowski = nixosSystem ./systems/SebastianJanikowski.nix;
}
```

Note: these attributes can also be nested as desired; however, the selected
attribute must be a valid, buildable NixOS system.

Explicit configurations are usually discovered in the following order for
most commands, unless otherwise specified:

- *$NIXOS_SYSTEM* (environment variable)
- *<nixos-system>* (_$NIX_PATH_ entry)
- *config_location* setting (this can also be a directory containing a 
  _system.nix_ file; does not support passing attributes)

Both *$NIXOS_SYSTEM* and the *<nixos-system>* entry in _$NIX_PATH_ can specify
an attribute to build by suffixing the path with a #. Example values for these:

- _/etc/nixos/system.nix_ (uses top-level attribute)
- _/path/to/system.nix#BoJackson_ (selects the BoJackson attribute)

## IMPLICIT CONFIGURATIONS

An *implicit configuration* is a path to a NixOS module that serves as the
entrypoint to a single NixOS system.

Unlike an explicit configuration, it always relies on a _<nixpkgs>_ entry in
_$NIX_PATH_ being present, and will automatically import _<nixpkgs/nixos>_ and
pass this module to it.

Implicit configurations are usually discovered in the following order for most
commands, unless otherwise specified:

- *$NIXOS_CONFIG* (environment variable)
- *<nixos-config>* (_$NIX_PATH_ entry)

# SEE ALSO

*nixos-cli-env(5)*

*nixos-cli-settings(5)*

# AUTHORS

Maintained by the *nixos-cli* team. See the main man page *nixos-cli(1)* for
details.
