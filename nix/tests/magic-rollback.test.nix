# This test is mostly copied from nixpkgs,
# courtesy of `nixos-rebuild-ng`.
{hostPkgs, ...}: {
  node.pkgsReadOnly = false;

  nodes = {
    deployer = {
      config,
      lib,
      pkgs,
      modulesPath,
      ...
    }: let
      inherit (import ./resources/ssh-keys.nix pkgs) snakeOilPrivateKey snakeOilPublicKey;
    in {
      imports = ["${modulesPath}/profiles/installation-device.nix"];

      # Disable the warning emitted by having multiple password
      # modes defined for the root user; it is already defined as
      # empty by the imported installation device module.
      users.users.root.initialHashedPassword = lib.mkForce null;

      programs.nixos-cli = {
        enable = true;
        package = pkgs.nixos-cli-legacy;
        settings = {
          ssh = {
            host_key_verification = "accept-new";
          };
          confirmation.always = true;
          root = {
            command = "sudo";
            password_method = "none";
          };
        };
        option-cache.enable = false;
      };

      system.includeBuildDependencies = true;

      systemd.services.ssh-agent = {
        description = "SSH key agent";
        wantedBy = ["default.target"];
        serviceConfig = {
          Type = "simple";
          Environment = [
            "SSH_AUTH_SOCK=/run/ssh-agent.socket"
          ];
          ExecStart = "${config.services.openssh.package}/bin/ssh-agent -D -a $SSH_AUTH_SOCK";
        };
      };

      # Define a single socket for the whole test,
      # since everything runs as root.
      environment.variables = {
        SSH_AUTH_SOCK = "/run/ssh-agent.socket";
      };

      nix.settings = {
        substituters = lib.mkForce [];
        hashed-mirrors = null;
        connect-timeout = 1;
      };

      virtualisation = {
        cores = 2;
        memorySize = 3072;
      };

      system.build.privateKey = snakeOilPrivateKey;
      system.build.publicKey = snakeOilPublicKey;
      system.switch.enable = true;
    };

    target = {
      nodes,
      lib,
      ...
    }: let
      targetConfig = {
        documentation.enable = false;
        services.openssh.enable = true;

        users.users.root.openssh.authorizedKeys.keys = [nodes.deployer.system.build.publicKey];
        users.users.alice.openssh.authorizedKeys.keys = [nodes.deployer.system.build.publicKey];
        users.users.bob.openssh.authorizedKeys.keys = [nodes.deployer.system.build.publicKey];

        users.users.alice.extraGroups = ["wheel"];
        users.users.bob.extraGroups = ["wheel"];

        # Disable sudo for root to ensure sudo isn't called without `--sudo`
        security.sudo.extraRules = lib.mkForce [
          {
            groups = ["wheel"];
            commands = [{command = "ALL";}];
          }
          {
            users = ["alice"];
            commands = [
              {
                command = "ALL";
                options = ["NOPASSWD"];
              }
            ];
          }
        ];

        nix.settings.trusted-users = ["@wheel"];
      };
    in {
      config = lib.mkMerge [
        targetConfig
        {
          users.users.alice = {
            isNormalUser = true;
            description = "Alice Foobar";
            password = "foobar";
            uid = 1000;
          };

          users.users.bob = {
            isNormalUser = true;
            description = "Bob Foobar";
            password = "foobar";
          };

          system.build = {
            inherit targetConfig;
          };
          system.switch.enable = true;

          networking.hostName = "target";
        }
      ];
    };
  };

  testScript = {nodes, ...}: let
    sshConfig = builtins.toFile "ssh.conf" ''
      UserKnownHostsFile=/dev/null
      StrictHostKeyChecking=no
    '';

    targetConfigJSON = hostPkgs.writeText "target-configuration.json" (
      builtins.toJSON nodes.target.system.build.targetConfig
    );

    targetNetworkJSON = hostPkgs.writeText "target-network.json" (
      builtins.toJSON nodes.target.system.build.networkConfig
    );

    configFile = hostname: extraConfig:
      hostPkgs.writeText "configuration.nix" # nix
      
      ''
        { lib, modulesPath, ... }: {
          imports = [
            (modulesPath + "/virtualisation/qemu-vm.nix")
            (modulesPath + "/testing/test-instrumentation.nix")
            (modulesPath + "/../tests/common/user-account.nix")
            (lib.modules.importJSON ./target-configuration.json)
            (lib.modules.importJSON ./target-network.json)
            ./hardware-configuration.nix
          ];

          boot.loader.grub = {
            enable = true;
            device = "/dev/vda";
            forceInstall = true;
          };

          ${extraConfig}

          # this will be asserted
          networking.hostName = "${hostname}";
        }
      '';

    initialConfig = configFile "initial-config" "";

    passwordlessSudoSuccess = configFile "passwordless-sudo" "";

    sshWasDisabled =
      configFile "ssh-was-disabled" # nix
      
      ''
        services.openssh.enable = lib.mkForce false;
      '';

    failingActivation =
      configFile "failing-activation" # nix
      
      ''
        system.activationScripts.failure = {
          text = "exit 1";
        };
      '';
  in
    # python
    ''
      start_all()
      target.wait_for_open_port(22)

      deployer.wait_until_succeeds("ping -c1 target")
      deployer.succeed("install -Dm 600 ${nodes.deployer.system.build.privateKey} /root/.ssh/target_access_key")
      deployer.succeed("install ${sshConfig} /root/.ssh/config")

      deployer.succeed("ssh-add /root/.ssh/target_access_key")

      target.succeed("nixos-generate-config")
      deployer.succeed("scp alice@target:/etc/nixos/hardware-configuration.nix /root/hardware-configuration.nix")

      deployer.copy_from_host("${initialConfig}", "/root/initial-config.nix")
      deployer.copy_from_host("${passwordlessSudoSuccess}", "/root/passwordless-sudo.nix")
      deployer.copy_from_host("${sshWasDisabled}", "/root/ssh-disabled.nix")
      deployer.copy_from_host("${failingActivation}", "/root/failing-activation.nix")

      deployer.copy_from_host("${targetNetworkJSON}", "/root/target-network.json")
      deployer.copy_from_host("${targetConfigJSON}", "/root/target-configuration.json")

      # Ensure sudo is disabled for root
      target.fail("sudo true")

      with subtest("Deploy to root@target successfully"):
        deployer.succeed("nixos apply -v -I nixos-config=/root/initial-config.nix --target-host alice@target --remote-root")
        target_hostname = deployer.succeed("ssh alice@target cat /etc/hostname", timeout=1).rstrip()
        assert target_hostname == "initial-config", f"{target_hostname=}"

      with subtest("Deploy to alice@target successfully using passwordless sudo"):
        deployer.succeed("nixos apply -v -I nixos-config=/root/passwordless-sudo.nix --target-host alice@target --remote-root")
        target_hostname = deployer.succeed("ssh alice@target cat /etc/hostname", timeout=1).rstrip()
        assert target_hostname == "passwordless-sudo", f"{target_hostname=}"

      with subtest("Attempt to disable SSH should trigger magic rollback"):
        deployer.fail("nixos apply -v -I nixos-config=/root/ssh-disabled.nix --rollback-timeout=3s --target-host root@target")
        target_hostname = deployer.succeed("ssh alice@target cat /etc/hostname", timeout=1).rstrip()
        assert target_hostname == "passwordless-sudo", f"{target_hostname=}"

      with subtest("Failing activation script should trigger magic rollback"):
        deployer.fail("nixos apply -v -I nixos-config=/root/failing-activation.nix --rollback-timeout=3s --target-host root@target")
        target_hostname = deployer.succeed("ssh alice@target cat /etc/hostname", timeout=1).rstrip()
        assert target_hostname == "passwordless-sudo", f"{target_hostname=}"

      # TODO: add concurrent supervisors test failure test?
    '';
}
