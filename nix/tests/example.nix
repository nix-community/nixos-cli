{
  pkgs,
  self,
}:
pkgs.testers.runNixOSTest {
  name = "example-test";
  nodes.machine1 = _: {
    imports = [self.nixosModules.nixos-cli];
    services.nixos-cli.enable = true;
  };

  testScript = ''
    machine1.succeed("nixos features")
  '';
}
