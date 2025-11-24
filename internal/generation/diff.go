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
	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/nix-community/nixos-cli/internal/system"
)

type DiffCommandOptions struct {
	DiffTool    settings.DiffTool
	DiffToolCmd []string
}

func RunDiffCommand(s system.System, before string, after string, opts *DiffCommandOptions) error {
	log := s.Logger()

	tool := opts.DiffTool

	switch tool {
	case settings.DifferInternal:
		if s.IsRemote() {
			log.Warn("the internal differ does not work with remote systems")
			tool = settings.DifferNix
		}
	case settings.DifferCommand:
		cmd := opts.DiffToolCmd[0]
		if cmdPath, _ := exec.LookPath(cmd); cmdPath == "" {
			log.Warnf("differ.command uses '%s', but `%s` is not executable", cmd, cmd)
			tool = settings.DifferNix
		}
	case settings.DifferNvd:
		if nvdPath, _ := exec.LookPath("nvd"); nvdPath == "" {
			log.Warn("differ.tool is set to 'nvd', but `nvd` is not executable")
			tool = settings.DifferNix
		}
	}

	if opts.DiffTool != tool && tool == settings.DifferNix {
		log.Warn("falling back to `nix store diff-closures`")
	}

	var argv []string

	switch tool {
	case settings.DifferCommand:
		argv = append(opts.DiffToolCmd, before, after)
	case settings.DifferInternal:
		diffs, err := diffNixStoreDB(before, after)
		if err != nil {
			return err
		}
		_ = diffs
		log.Info("internal differ currently not implemented")
		return nil
	case settings.DifferNix:
		argv = []string{"nix", "store", "diff-closures", before, after}
	case settings.DifferNvd:
		argv = []string{"nvd", "diff", before, after}
	}

	s.Logger().CmdArray(argv)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	_, err := s.Run(cmd)
	return err
}

type PathInfo struct {
	Name    string
	Version string
}

type SystemPathStatus string

const (
	SystemPathStatusBoth    SystemPathStatus = "both"
	SystemPathStatusNeither SystemPathStatus = "neither"
	SystemPathStatusOldOnly SystemPathStatus = "old-only"
	SystemPathStatusNewOnly SystemPathStatus = "new-only"
)

type PathDiff struct {
	Name             string
	Old              []string
	New              []string
	Change           ChangeType
	SystemPathStatus SystemPathStatus
}

type ChangeType string

const (
	ChangeTypeAdd    ChangeType = "add"
	ChangeTypeRemove ChangeType = "remove"
	ChangeTypeChange ChangeType = "change"
)

func diffNixStoreDB(before string, after string) ([]PathDiff, error) {
	conn, err := sql.Open("sqlite", constants.NixStoreDatabase)
	if err != nil {
		return nil, fmt.Errorf("error opening nix sqlite db: %w", err)
	}
	defer func() { _ = conn.Close() }()

	closuresBefore, err := getClosure(conn, before)
	if err != nil {
		return nil, err
	}

	systemPathsBefore, err := getSystemPathDrvPaths(conn, before)
	if err != nil {
		return nil, err
	}

	closuresAfter, err := getClosure(conn, after)
	if err != nil {
		return nil, err
	}

	systemPathsAfter, err := getSystemPathDrvPaths(conn, after)
	if err != nil {
		return nil, err
	}

	diffs := calculateDiffs(
		closuresBefore.Paths,
		closuresAfter.Paths,
		systemPathsBefore,
		systemPathsAfter,
	)

	return diffs, nil
}

type Closure struct {
	Size  uint64
	Paths []PathInfo
}

// Retrieve ALL the transitive dependencies of a closure in the Nix store,
// as well as the total size of the closure.
//
// This includes all paths, so if a caller wants to differentiate between
// user-added packages and intermediate derivations such as Nix store
// paths, they should use get
func getClosure(conn *sql.DB, closurePath string) (*Closure, error) {
	resolvedClosurePath, err := filepath.EvalSymlinks(closurePath)
	if err != nil {
		return nil, err
	}

	// Credit to https://github.com/faukah/dix for this query.
	const query = `
WITH RECURSIVE
closure (id) AS (
    SELECT id
    FROM ValidPaths
    WHERE path = ?
    UNION
    SELECT r.reference FROM Refs r
    JOIN closure c ON r.referrer = c.id
)

SELECT v.path, v.narSize FROM closure c
JOIN ValidPaths v ON v.id = c.id;
`

	rows, err := conn.Query(query, resolvedClosurePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []PathInfo
	var totalSize uint64

	for rows.Next() {
		var path string
		var size uint64

		err = rows.Scan(&path, &size)
		if err != nil {
			return &Closure{
				Paths: results,
				Size:  totalSize,
			}, fmt.Errorf("error scanning rows: %w", err)
		}

		totalSize += size

		pname, version := parsePnameAndVersion(path)

		results = append(results, PathInfo{
			Name:    pname,
			Version: version,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error scanning rows: %w", err)
	}

	return &Closure{
		Size:  totalSize,
		Paths: results,
	}, nil
}

// Get the paths that are in the NixOS system closure's `system-path` derivation.
//
// These is the set of packages that are explicitly added using
// `environment.systemPackages` and friends, which is usually
// what users add packages to and would like to see updates for.
func getSystemPathDrvPaths(conn *sql.DB, closurePath string) ([]PathInfo, error) {
	resolvedClosurePath, err := filepath.EvalSymlinks(closurePath)
	if err != nil {
		return nil, err
	}

	// Credit to https://github.com/faukah/dix for this query.
	const query = `
WITH
system_drv AS (
    SELECT id FROM validpaths
    WHERE path = ?
),

system_path AS (
    SELECT reference AS id FROM system_drv sd
    JOIN refs ON sd.id = referrer
    JOIN validpaths vp ON reference = vp.id
    WHERE (vp.path LIKE '%-system-path')
),

pkgs AS (
    SELECT reference AS id FROM refs
    JOIN system_path ON referrer = id
)

SELECT path FROM pkgs
JOIN validpaths vp ON vp.id = pkgs.id;
`

	rows, err := conn.Query(query, resolvedClosurePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var results []PathInfo

	for rows.Next() {
		var path string

		err = rows.Scan(&path)
		if err != nil {
			return results, fmt.Errorf("error scanning rows: %w", err)
		}

		pname, version := parsePnameAndVersion(path)

		results = append(results, PathInfo{
			Name:    pname,
			Version: version,
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

// Create a changeset between two different lists of store paths,
// usually calculated from the transitive closures and the sets.
// of system-path paths for that closure.
func calculateDiffs(
	oldPaths []PathInfo,
	newPaths []PathInfo,
	oldSystemPath []PathInfo,
	newSystemPath []PathInfo,
) []PathDiff {
	oldSystemPathSet := make(map[string]struct{})
	for _, path := range oldSystemPath {
		oldSystemPathSet[path.Name] = struct{}{}
	}

	newSystemPathSet := make(map[string]struct{})
	for _, path := range newSystemPath {
		newSystemPathSet[path.Name] = struct{}{}
	}

	diffSet := make(map[string]PathDiff, max(len(oldPaths), len(newPaths)))

	for _, path := range oldPaths {
		v := path.Version
		if v == "" {
			v = "<none>"
		}

		diff := diffSet[path.Name]
		diff.Old = append(diff.Old, v)
		diffSet[path.Name] = diff
	}

	for _, path := range newPaths {
		v := path.Version
		if v == "" {
			v = "<none>"
		}

		diff := diffSet[path.Name]
		diff.New = append(diff.New, v)
		diffSet[path.Name] = diff
	}

	diffs := make([]PathDiff, 0, len(diffSet))
	for name, diff := range diffSet {
		diff.Name = name

		if len(diff.Old) == 0 && len(diff.New) == 0 {
			continue
		}

		changed := false
		if len(diff.Old) > 0 && len(diff.New) > 0 {
			if diff.Old[0] != diff.New[0] {
				changed = true
			}
		}

		if len(diff.Old) > 0 && len(diff.New) == 0 {
			diff.Change = ChangeTypeRemove
		} else if len(diff.Old) == 0 && len(diff.New) > 0 {
			diff.Change = ChangeTypeAdd
		} else if changed {
			diff.Change = ChangeTypeChange
		}

		oldInPath := false
		newInPath := false

		if _, ok := oldSystemPathSet[name]; ok {
			oldInPath = true
		}
		if _, ok := newSystemPathSet[name]; ok {
			newInPath = true
		}

		if oldInPath && newInPath {
			diff.SystemPathStatus = SystemPathStatusBoth
		} else if !oldInPath && !newInPath {
			diff.SystemPathStatus = SystemPathStatusNeither
		} else if oldInPath && !newInPath {
			diff.SystemPathStatus = SystemPathStatusOldOnly
		} else if !oldInPath && newInPath {
			diff.SystemPathStatus = SystemPathStatusNewOnly
		}

		diffs = append(diffs, diff)
	}

	return diffs
}
