# Copied from nixpkgs test resources!
pkgs: {
  # This key is used in integration tests
  # This is NOT a security issue
  # It uses the same key as the one used in OpenSSH fuzz tests
  # https://github.com/openssh/openssh-portable/blob/V_9_9_P2/regress/misc/fuzz-harness/fixed-keys.h#L76-L85
  snakeOilPrivateKey = pkgs.writeText "privkey.snakeoil" ''
    -----BEGIN OPENSSH PRIVATE KEY-----
    b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
    QyNTUxOQAAACAz0F5hFTFS5nhUcmnyjFVoDw5L/P7kQU8JnBA2rWczAwAAAIhWlP99VpT/
    fQAAAAtzc2gtZWQyNTUxOQAAACAz0F5hFTFS5nhUcmnyjFVoDw5L/P7kQU8JnBA2rWczAw
    AAAEDE1rlcMC0s0X3TKVZAOVavZOywwkXw8tO5dLObxaCMEDPQXmEVMVLmeFRyafKMVWgP
    Dkv8/uRBTwmcEDatZzMDAAAAAAECAwQF
    -----END OPENSSH PRIVATE KEY-----
  '';

  snakeOilPublicKey = "ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIDPQXmEVMVLmeFRyafKMVWgPDkv8/uRBTwmcEDatZzMD snakeoil";
}
