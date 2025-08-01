{
  lib,
  buildGoModule,
  nix-gitignore,
  installShellFiles,
  buildPackages,
  stdenv,
  scdoc,
  revision ? "unknown",
  flake ? true,
}:
buildGoModule (finalAttrs: {
  pname = "nixos-cli";
  version = "0.13.0-dev";
  src = nix-gitignore.gitignoreSource [] ./.;

  vendorHash = "sha256-KQTzKdP9If3Px2nLTjYks6HQxpa7nrLAu8FU49LUPto=";

  nativeBuildInputs = [installShellFiles scdoc];

  env =
    {
      CGO_ENABLED = 0;
      COMMIT_HASH = revision;
      FLAKE = lib.boolToString flake;
      VERSION = finalAttrs.version;
    }
    // (lib.optionalAttrs stdenv.isLinux {
      SYSTEMD_DBUS_INTERFACE_DIR = "${buildPackages.systemd}/share/dbus-1/interfaces";
    });

  buildPhase = ''
    runHook preBuild

    # Force-regenerate the dbus bindings. They can be different depending
    # on the version of `nixpkgs`, and builds must fail if the interface
    # does not match anymore.
    make gen-dbus-bindings

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
  };
})
