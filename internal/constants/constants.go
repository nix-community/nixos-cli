package constants

const (
	NixProfileDirectory       = "/nix/var/nix/profiles"
	NixSystemProfileDirectory = NixProfileDirectory + "/system-profiles"
	DefaultConfigLocation     = "/etc/nixos-cli/config.toml"
	CurrentSystem             = "/run/current-system"
	NixOSMarker               = "/etc/NIXOS"
	NixChannelDirectory       = NixProfileDirectory + "/per-user/root/channels"
	NixOSVersionFile          = "nixos-version"
	NixStoreDatabase          = "/nix/var/nix/db/db.sqlite"
)
