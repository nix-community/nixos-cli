{
  lib,
  buildGoModule,
}:
buildGoModule (finalAttrs: {
  pname = "nixos-cli-activation-supervisor";
  version = "0.15.0-dev";

  src = lib.fileset.toSource {
    root = ../.;
    fileset = lib.fileset.unions [
      ../go.mod
      ../go.sum
      ../Makefile
      ../cmd
      ../doc
      ../internal
      ../supervisor
    ];
  };

  vendorHash = "sha256-JjCXGy/ysZajWSZP2Ap90iSbyQaIifa+l16x7215d+c=";

  env = {
    CGO_ENABLED = 0;
    VERSION = finalAttrs.version;
  };

  buildPhase = ''
    runHook preBuild
    make supervisor
    runHook postBuild
  '';

  installPhase = ''
    runHook preInstall
    install -Dm755 ./activation-supervisor $out/bin/activation-supervisor
    runHook postInstall
  '';

  meta = with lib; {
    homepage = "https://github.com/nix-community/nixos";
    description = "Activation/rollback supervisor used by nixos-cli";
    license = licenses.gpl3Only;
    maintainers = with maintainers; [water-sucks];
    mainProgram = "activation-supervisor";
  };
})
