{
  lib,
  symlinkJoin,
  makeBinaryWrapper,
  nix,
  nixos-cli-unwrapped,
}: let
  runtimeDeps = [
    # Make sure that we use the Nix package we depend on, not something
    # else from the PATH for nix-{env,instantiate,build}. This is
    # important, because NixOS defaults the architecture of the rebuilt
    # system to the architecture of the nix-* binaries used. So if on an
    # amd64 system the user has an i686 Nix package in her PATH, then we
    # would silently downgrade the whole system to be i686 NixOS on the
    # next reboot.
    # Credit where credit is due: this note is taken directly from nixos-rebuild-ng.
    nix
  ];
in
  symlinkJoin {
    pname = "nixos-cli";
    inherit (nixos-cli-unwrapped) version meta;

    paths = [
      nixos-cli-unwrapped
    ];

    nativeBuildInputs = [
      makeBinaryWrapper
    ];

    postBuild = ''
      wrapProgram $out/bin/nixos \
        --prefix PATH : ${lib.makeBinPath runtimeDeps}
    '';
  }
