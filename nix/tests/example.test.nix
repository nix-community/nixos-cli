{
  pkgs,
  self,
  ...
}:
pkgs.testers.runNixOSTest {
  name = "example-test";
  defaults = {
    imports = [self.nixosModules.nixos-cli];
    services.nixos-cli.enable = true;
    services.nixos-cli.useActivationInterface = true;
    services.nixos-cli.magic-rollback.enable = true;
  };
  nodes.machine1 = _: {};

  testScript = ''
    machine1.succeed("nixos features")
  '';
}
