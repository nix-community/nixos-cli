{
  pkgs ? import <nixpkgs> {},
  self ? (import ../flake-compat.nix).outputs,
}: let
  inherit (pkgs) lib;

  mkTest = test: import test {inherit pkgs self;};

  findTests = dir: let
    entries =
      lib.genAttrs' (lib.filesystem.listFilesRecursive dir)
      (name: lib.nameValuePair (builtins.toString name) (builtins.toString name));

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
  lib.mapAttrs (_: test: mkTest test) tests
