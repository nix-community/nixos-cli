package generation

import (
	"database/sql"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	_ "modernc.org/sqlite"

	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/system"
)

type DiffCommandOptions struct {
	UseNvd bool
}

func RunDiffCommand(s system.CommandRunner, before string, after string, opts *DiffCommandOptions) error {
	log := s.Logger()

	useNvd := opts.UseNvd

	if opts.UseNvd {
		nvdPath, _ := exec.LookPath("nvd")
		nvdFound := nvdPath != ""
		if !nvdFound {
			log.Warn("use_nvd is specified in config, but `nvd` is not executable")
			log.Warn("falling back to `nix store diff-closures`")
			useNvd = false
		}
	}

	argv := []string{"nix", "store", "diff-closures", before, after}
	if useNvd {
		argv = []string{"nvd", "diff", before, after}
	}

	s.Logger().CmdArray(argv)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	_, err := s.Run(cmd)
	return err
}

type ClosureDiff struct {
	Added   []PathInfo
	Removed []PathInfo
	Changed struct {
		Old []PathInfo
		New []PathInfo
	}
}

type PathInfo struct {
	Name    string
	Version string
	Hash    string
	Size    int
}

func DiffNixStoreDB(before string, after string) (*ClosureDiff, error) {
	conn, err := sql.Open("sqlite", constants.NixStoreDatabase)
	if err != nil {
		return nil, fmt.Errorf("error opening nix sqlite db: %w", err)
	}
	defer func() { _ = conn.Close() }()

	closuresBefore, err := getClosurePaths(conn, before)
	if err != nil {
		return nil, err
	}

	closuresAfter, err := getClosurePaths(conn, after)
	if err != nil {
		return nil, err
	}

	return diffClosureSets(closuresBefore, closuresAfter), nil
}

func diffClosureSets(before, after []PathInfo) *ClosureDiff {
	diff := &ClosureDiff{}

	key := func(p PathInfo) string {
		if p.Version != "" {
			return p.Name + ":" + p.Version
		}
		return p.Name
	}

	beforeMap := make(map[string]PathInfo, len(before))
	afterMap := make(map[string]PathInfo, len(after))

	for _, p := range before {
		beforeMap[key(p)] = p
	}
	for _, p := range after {
		afterMap[key(p)] = p
	}

	for k, oldPath := range beforeMap {
		newPath, exists := afterMap[k]
		if !exists {
			diff.Removed = append(diff.Removed, oldPath)
			continue
		}

		if oldPath.Hash != newPath.Hash {
			diff.Changed.Old = append(diff.Changed.Old, oldPath)
			diff.Changed.New = append(diff.Changed.New, newPath)
		}
	}

	for k, new := range afterMap {
		if _, exists := beforeMap[k]; !exists {
			diff.Added = append(diff.Added, new)
		}
	}

	return diff
}

func getClosurePaths(conn *sql.DB, closurePath string) ([]PathInfo, error) {
	resolvedClosurePath, err := filepath.EvalSymlinks(closurePath)
	if err != nil {
		return nil, err
	}

	const query = `
WITH RECURSIVE closure(id) AS (
    SELECT id FROM ValidPaths WHERE path = ?
    UNION
    SELECT R.reference
    FROM Refs R JOIN closure C ON R.referrer = C.id
)
SELECT id, path, hash, narSize FROM ValidPaths WHERE id IN closure;
`

	rows, err := conn.Query(query, resolvedClosurePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []PathInfo

	for rows.Next() {
		var id int
		var path string
		var hash string
		var size int

		err = rows.Scan(&id, &path, &hash, &size)
		if err != nil {
			return results, fmt.Errorf("error scanning rows: %w", err)
		}

		pname, version := parsePnameAndVersion(path)
		results = append(results, PathInfo{
			Name:    pname,
			Version: version,
			Hash:    hash,
			Size:    size,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error scanning rows: %w", err)
	}

	return results, nil
}

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
