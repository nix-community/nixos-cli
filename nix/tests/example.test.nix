{
  pkgs,
  self,
  ...
}:
pkgs.testers.runNixOSTest {
  name = "example-test";
  nodes.machine1 = _: {
    imports = [self.nixosModules.nixos-cli];
    programs.nixos-cli.enable = true;
  };

  testScript = ''
    machine1.succeed("nixos features")
  '';
}
