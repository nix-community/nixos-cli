package build

import (
	"github.com/nix-community/nixos-cli/internal/build/vars"
)

func boolCheck(varName string, value string) {
	if value != "true" && value != "false" {
		panic("Compile-time variable internal.build." + varName + " is not a value of either 'true' or 'false'; this application was compiled incorrectly")
	}
}

func boolCast(value string) bool {
	switch value {
	case "true":
		return true
	case "false":
		return false
	default:
		panic("unreachable, this variable has not been bool-checked properly")
	}
}

func Version() string {
	return vars.Version
}

func GitRevision() string {
	return vars.GitRevision
}

func Flake() bool {
	return boolCast(vars.Flake)
}

func NixpkgsVersion() string {
	return vars.NixpkgsVersion
}

func init() {
	boolCheck("Flake", vars.Flake)
}
