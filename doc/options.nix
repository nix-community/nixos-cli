{pkgs ? import <nixpkgs> {}}: let
  self = (import ../nix/flake-compat.nix).outputs;

  inherit (self.inputs) nixpkgs;

  nixosModules = import "${nixpkgs}/nixos/modules/module-list.nix";

  optnixLib = self.inputs.optnix.mkLib pkgs;
in
  optnixLib.mkOptionsListFromModules {
    modules =
      nixosModules
      ++ [
        self.nixosModules.nixos-cli
      ];
    specialArgs = {
      inherit pkgs;
    };
    excluded = ["_module.args"];
  }
