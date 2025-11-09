{
  description = "A unified NixOS tooling replacement for nixos-* utilities";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

    flake-compat = {
      url = "github:edolstra/flake-compat";
      flake = false;
    };

    nix-options-doc.url = "github:Thunderbottom/nix-options-doc/v0.2.0";
  };

  outputs = {
    self,
    nixpkgs,
    ...
  } @ inputs: let
    inherit (nixpkgs) lib;
    eachSystem = lib.genAttrs lib.systems.flakeExposed;
    pkgsFor = system: nixpkgs.legacyPackages.${system};
  in {
    packages = eachSystem (system: let
      pkgs = pkgsFor system;
      inherit (pkgs) callPackage;
    in {
      default = self.packages.${system}.nixos;

      nixos = callPackage ./package.nix {
        revision = self.rev or self.dirtyRev or "unknown";
      };
      nixosLegacy = self.packages.${system}.nixos.override {flake = false;};
    });

    devShells = eachSystem (system: let
      pkgs = pkgsFor system;
      inherit (pkgs) go golangci-lint mkShell mdbook scdoc;
      inherit (pkgs.nodePackages) prettier;

      nix-options-doc = inputs.nix-options-doc.packages.${system}.default;
    in {
      default = mkShell {
        name = "nixos-shell";
        nativeBuildInputs = [
          go
          golangci-lint

          mdbook
          prettier
          scdoc
          nix-options-doc
        ];
      };
    });

    nixosModules.nixos-cli = lib.modules.importApply ./module.nix {inherit self;};
  };
}
