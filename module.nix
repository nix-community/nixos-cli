{
  self,
  useFlakePkg ? true,
}: {
  options,
  config,
  pkgs,
  lib,
  ...
}: let
  cfg = config.services.nixos-cli;
  nixosCfg = config.system.nixos;

  inherit (pkgs.stdenv.hostPlatform) system;

  inherit (lib) types;

  tomlFormat = pkgs.formats.toml {};
in {
  options.services.nixos-cli = {
    enable = lib.mkEnableOption "unified NixOS tooling replacement for nixos-* utilities";

    package = lib.mkOption {
      type = types.package;
      default =
        if useFlakePkg
        then self.packages.${system}.nixos
        else self.packages.${system}.nixosLegacy;
      description = "Package to use for nixos-cli";
    };

    config = lib.mkOption {
      type = tomlFormat.type;
      default = {};
      description = "Configuration for nixos-cli, in TOML format";
      apply = prev: let
        # Inherit this from the old nixos-generate-config attrs. Easy to deal with, for now.
        desktopConfig = lib.concatStringsSep "\n" config.system.nixos-generate-config.desktopConfiguration;
      in
        lib.recursiveUpdate {
          init = {
            xserver_enabled = config.services.xserver.enable;
            desktop_config = desktopConfig;
            extra_config = "";
          };
        }
        prev;
    };

    useActivationInterface = lib.mkOption {
      type = types.bool;
      default = false;
      example = true;
      description = ''
        Use the `nixos activate` interface to switch configurations, instead of the
        `switch-to-configuration-ng` program that is currently used in `nixpkgs`.

        The behavior is mostly the same, but changes can be made that may potentially
        break behavior from the original on a per-case basis.

        If this is disabled, users will still be able to use `nixos activate` on their
        own, but it will serve solely as a shim to run the switch script on a switchable.
        system.

        This activation interface is experimental and subject to change.
      '';
    };

    generationTag = lib.mkOption {
      type = types.nullOr types.str;
      default = lib.maybeEnv "NIXOS_GENERATION_TAG" null;
      description = "A description for this generation";
      example = "Sign Git GPG commits by default";
    };

    prebuildOptionCache = lib.mkOption {
      type = types.bool;
      default = config.documentation.nixos.enable;
      description = "Prebuild JSON cache for `nixos option` command";
    };
  };

  config = lib.mkIf cfg.enable (lib.mkMerge [
    {
      environment.systemPackages = [cfg.package];

      environment.etc."nixos-cli/config.toml".source =
        tomlFormat.generate "nixos-cli-config.toml" cfg.config;

      # Hijack system builder commands to insert a `nixos-version.json` file at the root.
      system.systemBuilderCommands = let
        nixos-version-json = builtins.toJSON {
          nixosVersion = "${nixosCfg.distroName} ${nixosCfg.release} (${nixosCfg.codeName})";
          nixpkgsRevision = nixosCfg.revision;
          configurationRevision = "${builtins.toString config.system.configurationRevision}";
          description = cfg.generationTag;
        };
      in ''
        cat > "$out/nixos-version.json" << EOF
        ${nixos-version-json}
        EOF
      '';

      # FIXME: should this be configurable? Not all users would want to preserve
      # SSH_AUTH_SOCK, for example.
      security.sudo.extraConfig = ''
        # Preserve NIXOS_CONFIG and NIXOS_CLI_CONFIG in sudo invocations of
        # `nixos apply`. This is required in order to keep ownership across
        # automatic re-exec as root.
        Defaults env_keep += "NIXOS_CONFIG"
        Defaults env_keep += "NIXOS_GENERATION_TAG"
        Defaults env_keep += "NIXOS_CLI_CONFIG"
        Defaults env_keep += "NIXOS_CLI_DISABLE_STEPS"
        Defaults env_keep += "NIXOS_CLI_DEBUG_MODE"
        Defaults env_keep += "NIXOS_CLI_SUPPRESS_NO_SETTINGS_WARNING"
        Defaults env_keep += "SSH_AUTH_SOCK"
      '';
    }
    (lib.mkIf cfg.prebuildOptionCache {
      # While there is already an `options.json` that exists in the
      # `config.system.build.manual.optionsJSON` attribute, this is
      # not as full-featured, because it does not contain NixOS options
      # that are not available in base `nixpkgs`. This does increase
      # eval time, but that's a fine tradeoff in this case since it
      # is able to be disabled.
      environment.etc."nixos-cli/options-cache.json" = {
        text = let
          optionList' = lib.optionAttrSetToDocList options;
          optionList = builtins.filter (v: v.visible && !v.internal) optionList';
        in
          builtins.toJSON optionList;
      };
    })
    (lib.mkIf cfg.useActivationInterface {
      # This looks confusing, but this only stops the switch-to-configuration-ng
      # program from being used. The system will still be switchable.
      system.switch.enable = lib.mkForce false;

      # Use a subshell so we can source makeWrapper's setup hook without
      # affecting the rest of activatableSystemBuilderCommands.
      system.activatableSystemBuilderCommands = ''
        (
          source ${pkgs.buildPackages.makeWrapper}/nix-support/setup-hook

          mkdir $out/bin

          ln -sf ${lib.getExe cfg.package} $out/bin/switch-to-configuration

          wrapProgram $out/bin/switch-to-configuration \
            --add-flags activate \
            --set NIXOS_CLI_ATTEMPTING_ACTIVATION 1 \
            --set OUT $out \
            --set TOPLEVEL ''${!toplevelVar} \
            --set DISTRO_ID ${lib.escapeShellArg config.system.nixos.distroId} \
            --set INSTALL_BOOTLOADER ${lib.escapeShellArg config.system.build.installBootLoader} \
            --set PRE_SWITCH_CHECK ${lib.escapeShellArg config.system.preSwitchChecksScript} \
            --set LOCALE_ARCHIVE ${config.i18n.glibcLocales}/lib/locale/locale-archive \
            --set SYSTEMD ${config.systemd.package}
        )
      '';
    })
  ]);
}
