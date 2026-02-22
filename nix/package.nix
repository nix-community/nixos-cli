{
  lib,
  buildGoModule,
  installShellFiles,
  stdenv,
  scdoc,
  nix,
  revision ? "unknown",
  flake ? true,
}:
buildGoModule (finalAttrs: {
  pname = "nixos-cli";
  version = "0.15.0-dev";

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../go.mod
      ../go.sum
      ../Makefile
      ../main.go
      ../cmd
      ../doc
      ../internal
    ];
  };

  vendorHash = "sha256-J4vibcWXzeBInb4CdNmqP8svYF2QX0Gccm/kiumQ4rA=";

  nativeBuildInputs = [installShellFiles scdoc];

  propagatedBuildInputs = [
    # Make sure that we use the Nix package we depend on, not something
    # else from the PATH for nix-{env,instantiate,build}. This is
    # important, because NixOS defaults the architecture of the rebuilt
    # system to the architecture of the nix-* binaries used. So if on an
    # amd64 system the user has an i686 Nix package in her PATH, then we
    # would silently downgrade the whole system to be i686 NixOS on the
    # next reboot.
    # Credit where credit is due: this note is taken directly from nixos-rebuild-ng.
    (lib.getBin nix)
  ];

  env = {
    CGO_ENABLED = 0;
    COMMIT_HASH = revision;
    FLAKE = lib.boolToString flake;
    VERSION = finalAttrs.version;
    NIXPKGS_REVISION = lib.trivial.release;
  };

  buildPhase = ''
    runHook preBuild
    make all gen-manpages
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall

    install -Dm755 ./nixos -t $out/bin

    mkdir -p $out/share/man/man1
    mkdir -p $out/share/man/man5
    find man -name '*.1' -exec cp {} $out/share/man/man1/ \;
    find man -name '*.5' -exec cp {} $out/share/man/man5/ \;

    runHook postInstall
  '';

  postInstall = lib.optionalString (stdenv.buildPlatform.canExecute stdenv.hostPlatform) ''
    installShellCompletion --cmd nixos \
      --bash <($out/bin/nixos completion bash) \
      --fish <($out/bin/nixos completion fish) \
      --zsh <($out/bin/nixos completion zsh)
  '';

  meta = with lib; {
    homepage = "https://github.com/nix-community/nixos";
    description = "A unified NixOS tooling replacement for nixos-* utilities";
    license = licenses.gpl3Only;
    maintainers = with maintainers; [water-sucks];
    mainProgram = "nixos";
  };
})
