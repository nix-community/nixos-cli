{self, ...}: {
  perSystem = {pkgs, ...}: {
    checks = import ./. {inherit pkgs self;};
  };
}
