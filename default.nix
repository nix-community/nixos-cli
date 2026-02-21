{pkgs ? import <nixpkgs> {}}: let
  flakeSelf = (import ./nix/flake-compat.nix).outputs;
  inherit (pkgs.stdenv.hostPlatform) system;
in {
  inherit
    (flakeSelf.packages.${system})
    nixos-cli
    nixos-cli-legacy
    # Also inherit aliases, to be removed later.
    nixos
    nixosLegacy
    ;

  # Do not use lib.importApply here for better error tracking, since
  # it causes an infinite recursion for a currently unknown reason.
  module = import ./nix/module.nix {
    self = flakeSelf;
    # If someone is using default.nix for imports, it's likely that
    # they will also be using the legacy package on their system.
    useFlakePkg = false;
  };
}
