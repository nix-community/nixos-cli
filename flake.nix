{
  description = "A unified NixOS tooling replacement for nixos-* utilities";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";

    flake-compat = {
      url = "github:edolstra/flake-compat";
      flake = false;
    };

    flake-parts.url = "github:hercules-ci/flake-parts";

    optnix = {
      url = "github:water-sucks/optnix";
      inputs = {
        flake-compat.follows = "flake-compat";
        nixpkgs.follows = "nixpkgs";
      };
    };
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
      }: let
        mkWrapped = unwrapped:
          pkgs.callPackage ./nix/package {
            nixos-cli-unwrapped = unwrapped;
          };

        nixos-cli-unwrapped = pkgs.callPackage ./nix/package/unwrapped.nix {
          revision = self.rev or self.dirtyRev or "unknown";
        };
        nixos-cli-legacy-unwrapped = nixos-cli-unwrapped.override {flake = false;};

        nixos-cli = mkWrapped nixos-cli-unwrapped;
        nixos-cli-legacy = mkWrapped nixos-cli-legacy-unwrapped;
      in {
        packages = {
          default = nixos-cli;

          inherit
            nixos-cli
            nixos-cli-legacy
            nixos-cli-unwrapped
            nixos-cli-legacy-unwrapped
            ;

          nixos = lib.warn "the 'nixos' package has been renamed to 'nixos-cli'" self'.packages.nixos-cli;
          nixosLegacy = lib.warn "the 'nixosLegacy' package has been renamed to 'nixos-cli-legacy'" self'.packages.nixos-cli-legacy;
        };

        devShells = let
          inherit (pkgs) golangci-lint mkShell mdbook scdoc govulncheck prettier;
          go = pkgs.go_1_26;
        in {
          default = mkShell {
            name = "nixos-shell";
            nativeBuildInputs = [
              go
              golangci-lint
              govulncheck

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
