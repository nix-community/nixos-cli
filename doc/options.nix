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
      # This missing package is preventing option evaluation from
      # occurring. This needs to be fixed upstream.
      pkgs = pkgs.extend (final: prev: {
        xf86-video-nested = prev.xf86_video_nested;
      });
    };
    excluded = [
      "_module.args"
    ];
  }
