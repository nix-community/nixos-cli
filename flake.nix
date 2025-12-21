{
  description = "A unified NixOS tooling replacement for nixos-* utilities";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

    flake-compat = {
      url = "github:edolstra/flake-compat";
      flake = false;
    };

    flake-parts.url = "github:hercules-ci/flake-parts";

    optnix.url = "github:water-sucks/optnix";
  };

  outputs = {
    self,
    flake-parts,
    ...
  } @ inputs: let
    lib = inputs.nixpkgs.lib;
  in
    flake-parts.lib.mkFlake {inherit inputs;} {
      imports = [
        ./nix/tests/flake-module.nix
      ];

      systems = lib.systems.flakeExposed;

      perSystem = {
        self',
        pkgs,
        ...
      }: {
        packages = {
          default = self'.packages.nixos;

          nixos = pkgs.callPackage ./nix/package.nix {
            revision = self.rev or self.dirtyRev or "unknown";
          };

          nixosLegacy = self'.packages.nixos.override {flake = false;};
        };

        devShells = let
          inherit (pkgs) go golangci-lint mkShell mdbook scdoc;
          inherit (pkgs.nodePackages) prettier;
        in {
          default = mkShell {
            name = "nixos-shell";
            nativeBuildInputs = [
              go
              golangci-lint

              mdbook
              prettier
              scdoc
            ];
          };
        };
      };

      flake = {
        nixosModules.nixos-cli = lib.modules.importApply ./nix/module.nix {inherit self;};
      };
    };
}
