{pkgs ? import <nixpkgs> {}}: let
  flakeSelf = import ./flake-compat.nix;
in {
  inherit (flakeSelf.packages.${pkgs.system}) nixos nixosLegacy;

  # Do not use lib.importApply here for better error tracking, since
  # it causes an infinite recursion for a currently unknown reason.
  module = import ./module.nix {
    self = flakeSelf;
    # If someone is using default.nix for imports, it's likely that
    # they will also be using the legacy package on their system.
    useFlakePkg = false;
  };
}
