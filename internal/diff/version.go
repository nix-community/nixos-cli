package diff

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/nix-community/nixos-cli/internal/utils/set"
)

var pnameVersionRegex = regexp.MustCompile(
	`^[a-z0-9]+-(.+?)(-([0-9].*?))?(\.drv)?$`,
)

// Extract the pname and version from a Nix store path
// using a heuristic.
//
// This follows the Nixpkgs convention of using a dash
// and digit immediately following the dash to indicate
// a version number. Otherwise, it will attempt to strip the
// hash and leave it at that.
func parsePnameAndVersion(path string) (string, string) {
	base := filepath.Base(filepath.Clean(path))

	if matches := pnameVersionRegex.FindStringSubmatch(base); matches != nil {
		pname := matches[1]
		version := matches[3] // may be empty
		return pname, version
	}

	_, name, found := strings.Cut(base, "-")
	if !found {
		return base, ""
	}

	return name, ""
}

func buildPackageVersions(paths []PathInfo) map[string]set.Set[string] {
	out := make(map[string]set.Set[string])

	for _, p := range paths {
		s, ok := out[p.Name]
		if !ok {
			s = set.New[string]()
			out[p.Name] = s
		}

		s.Add(p.Version)
	}

	return out
}
