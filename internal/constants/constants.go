package constants

const (
	CurrentSystem             = "/run/current-system"
	DefaultConfigLocation     = "/etc/nixos-cli/config.toml"
	NixChannelDirectory       = NixProfileDirectory + "/per-user/root/channels"
	NixOSActivationDirectory  = "/run/nixos"
	NixOSMarker               = "/etc/NIXOS"
	NixOSVersionFile          = "nixos-version"
	NixProfileDirectory       = "/nix/var/nix/profiles"
	NixStoreDatabase          = "/nix/var/nix/db/db.sqlite"
	NixSystemProfileDirectory = NixProfileDirectory + "/system-profiles"
)
