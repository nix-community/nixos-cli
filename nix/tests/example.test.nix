{
  nodes.machine1 = {
    programs.nixos-cli = {
      enable = true;
    };
  };

  testScript = ''
    machine1.succeed("nixos features")
  '';
}
