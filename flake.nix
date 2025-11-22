{
  description = "A unified NixOS tooling replacement for nixos-* utilities";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

    flake-compat = {
      url = "github:edolstra/flake-compat";
      flake = false;
    };

    flake-parts.url = "github:hercules-ci/flake-parts";

    nix-options-doc.url = "github:Thunderbottom/nix-options-doc/v0.2.0";
  };

  outputs = {
    self,
    flake-parts,
    ...
  } @ inputs: let
    lib = inputs.nixpkgs.lib;
  in
    flake-parts.lib.mkFlake {inherit inputs;} {
      systems = lib.systems.flakeExposed;

      perSystem = {
        self',
        pkgs,
        system,
        ...
      }: {
        packages = {
          default = self'.packages.nixos;

          nixos = pkgs.callPackage ./package.nix {
            revision = self.rev or self.dirtyRev or "unknown";
          };

          nixosLegacy = self'.packages.nixos.override {flake = false;};
        };

        devShells = let
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
        };
      };

      flake = {
        nixosModules.nixos-cli = lib.modules.importApply ./module.nix {inherit self;};
      };
    };
}
