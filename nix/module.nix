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

  # Remove an attribute at a given attribute set path.
  # Useful for excluding options.
  # Taken from https://git.sr.ht/~watersucks/optnix
  removeNestedAttrs = paths: set: lib.foldl' (s: p: removeAtPath (parsePath p) s) set paths;

  parsePath = pathStr: lib.splitString "." pathStr;

  removeAtPath = path: set:
    if path == []
    then set
    else let
      key = builtins.head path;
      rest = builtins.tail path;
      sub = set.${key} or null;
    in
      if rest == []
      then removeAttrs set [key]
      else if builtins.isAttrs sub
      then
        set
        // {
          ${key} = removeAtPath rest sub;
        }
      else set;
in {
  imports = [
    (lib.mkRenamedOptionModule
      ["services" "nixos-cli" "prebuildOptionCache"]
      ["services" "nixos-cli" "option-cache" "enable"])
    (lib.mkRenamedOptionModule
      ["services" "nixos-cli" "useActivationInterface"]
      ["services" "nixos-cli" "activation-interface" "enable"])
    (lib.mkRenamedOptionModule
      ["services" "nixos-cli" "generationTag"]
      ["services" "nixos-cli" "generation-tag"])
  ];

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

    activation-interface = {
      enable = lib.mkOption {
        type = types.bool;
        default = false;
        example = true;
        description = ''
          Use the `nixos activate` interface to switch configurations, instead of the
          `switch-to-configuration-ng` program that is currently used in `nixpkgs`.

          The behavior is mostly the same, but changes can be made that may potentially
          break behavior from the original on a per-case basis.

          If this is disabled, users will still be able to use `nixos activate` on their
          own, but it will serve solely as a shim to run the switch script on a switchable
          system.

          This activation interface is experimental and subject to change.
        '';
      };
    };

    option-cache = {
      enable = lib.mkOption {
        type = types.bool;
        default = config.documentation.nixos.enable;
        description = "Prebuild JSON cache for `nixos option` command";
      };

      exclude = lib.mkOption {
        type = with lib.types; listOf str;
        example = [
          "sops"
          "programs.optnix"
          "boot.loader.grub.zfsSupport"
        ];
        default = [];
        description = ''
          Exclude these options or option paths from being included in the
          pre-generated option cache.

          This is a recursive action; excluding a path of "services.openssh",
          for example, means ALL options underneath that attribute set path
          of `services.openssh` will be excluded.

          This is useful for skipping over options that do not evaluate
          properly, either due to upstream errors or some other unforeseen
          evaluation error.
        '';
      };
    };

    preserve-env = lib.mkOption {
      type = with lib.types; listOf str;
      default = [
        "NIXOS_CLI_CONFIG"
        "NIXOS_CLI_DEBUG_MODE"
        "NIXOS_CLI_DISABLE_STEPS"
        "NIXOS_CLI_SUPPRESS_NO_SETTINGS_WARNING"
        "NIXOS_CONFIG"
        "NIXOS_GENERATION_TAG"
        "SSH_AUTH_SOCK"
      ];
      description = ''
        Environment variables to persist through root elevation command invocations
        on this host.

        Supports automatically configuring `sudo` and `doas` environment rules only.
        `sudo` rules are configured as defaults, while `doas` environment variable
        rules are configured for the `:wheel` group only.

        If you need more flexible definitions for these rules, consider using
        the default value of `services.nixos-cli.preserve-env` as a reference list
        of default variables that should be preserved across this boundary.

        It is recommended to use this option for any extra environment variables to
        preserve if `apply.reexec_as_root` is set in `nixos-cli`'s settings.
      '';
    };

    generation-tag = lib.mkOption {
      type = types.nullOr types.str;
      default = lib.maybeEnv "NIXOS_GENERATION_TAG" null;
      description = "A description for this generation";
      example = "Sign Git GPG commits by default";
      internal = true;
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
          configurationRevision = "${toString config.system.configurationRevision}";
          description = cfg.generation-tag;
        };
      in ''
        cat > "$out/nixos-version.json" << EOF
        ${nixos-version-json}
        EOF
      '';
    }
    (lib.mkIf (builtins.length cfg.preserve-env > 0) {
      security.sudo.extraConfig = let
        envKeepLine = var: ''Defaults env_keep += "${var}"'';
        configLines = map envKeepLine cfg.preserve-env;
      in ''
        ${lib.concatStringsSep "\n" configLines}
      '';

      security.doas.extraRules = let
        isDefaultList = cfg.preserve-env == options.services.nixos-cli.preserve-env.default;

        # Remove SSH_AUTH_SOCK from the default list if it equals the
        # defaults, since SSH_AUTH_SOCK is present and kept by the default
        # doas rule set.
        # That way, people will not be confused if they set -SSH_AUTH_SOCK
        # in their own doas config and see it added here again when they
        # never configured the option explicitly.
        preserveEnvList =
          if isDefaultList
          then (builtins.filter (var: var != "SSH_AUTH_SOCK") cfg.preserve-env)
          else cfg.preserve-env;
      in [
        {
          groups = ["wheel"];
          setEnv = preserveEnvList;
        }
      ];
    })
    (lib.mkIf cfg.option-cache.enable {
      # While there is already an `options.json` that exists in the
      # `config.system.build.manual.optionsJSON` attribute, this is
      # not as full-featured, because it does not contain NixOS options
      # that are not available in base `nixpkgs`. This does increase
      # eval time, but that's a fine tradeoff in this case since it
      # is able to be disabled.
      environment.etc."nixos-cli/options-cache.json" = {
        text = let
          excludedOptionPaths = cfg.option-cache.exclude;
          options' = removeNestedAttrs excludedOptionPaths options;

          rawOptionsDocList = lib.optionAttrSetToDocList options';

          optionList = builtins.filter (v: v.visible && !v.internal) rawOptionsDocList;
        in
          builtins.unsafeDiscardStringContext (builtins.toJSON optionList);
      };
    })
    (lib.mkIf cfg.activation-interface.enable {
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
