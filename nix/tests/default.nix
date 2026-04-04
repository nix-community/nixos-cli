{
  self ? (import ../flake-compat.nix).outputs,
  pkgs ? self.inputs.nixpkgs.legacyPackages.${builtins.currentSystem},
  ...
}: let
  inherit (pkgs) lib;

  mkTest = name: test: let
    test' = pkgs.testers.runNixOSTest {
      name = "${name}-test";
      imports = [
        test
        {
          node.specialArgs = {
            inherit self;
          };
        }
      ];
      defaults = {
        imports = [self.nixosModules.nixos-cli];
        nixpkgs.overlays = [
          (_final: _prev: {
            inherit (self.packages.${pkgs.stdenv.hostPlatform.system}) nixos-cli nixos-cli-legacy;
          })
        ];
      };
    };
  in
    test';

  findTests = dir: let
    entries =
      lib.genAttrs' (lib.filesystem.listFilesRecursive dir)
      (name: lib.nameValuePair (toString name) (toString name));

    filenames =
      lib.mapAttrs'
      (n: v: lib.nameValuePair (lib.strings.removePrefix (toString dir + "/") n) v)
      entries;

    testFiles = let
      filtered' = lib.filterAttrs (n: v: lib.strings.hasSuffix ".test.nix" n) filenames;

      normalizeTestName = name:
        lib.strings.replaceString "/" "-"
        (lib.strings.removeSuffix ".test.nix" name);
    in
      lib.mapAttrs' (n: v: lib.nameValuePair (normalizeTestName n) v) filtered';
  in
    testFiles;

  tests = findTests ./.;
in
  lib.mapAttrs mkTest tests
