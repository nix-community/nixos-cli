package generation

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
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
		diff, err := diffNixStoreDB(before, after)
		if err != nil {
			return err
		}
		displayDiffResults(diff)
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

type ClosureDiff struct {
	OldSize uint64
	NewSize uint64
	Diffs   []PathDiff
}

type PathInfo struct {
	Path    string
	Name    string
	Version string
	Deriver *string
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

func diffNixStoreDB(before string, after string) (*ClosureDiff, error) {
	conn, err := sql.Open("sqlite", fmt.Sprintf("file:%s?mode=ro&immutable=1", constants.NixStoreDatabase))
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

	return &ClosureDiff{
		OldSize: closuresBefore.Size,
		NewSize: closuresAfter.Size,
		Diffs:   diffs,
	}, nil
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

SELECT v.path, v.deriver, v.narSize FROM closure c
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
		var deriver *string
		var size uint64

		err = rows.Scan(&path, &deriver, &size)
		if err != nil {
			return &Closure{
				Paths: results,
				Size:  totalSize,
			}, fmt.Errorf("error scanning rows: %w", err)
		}

		totalSize += size

		results = append(results, PathInfo{
			Path:    path,
			Deriver: deriver,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error scanning rows: %w", err)
	}

	g, _ := getDrvAttrs(results)
	fillPnameVersion(results, g)

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

SELECT path, deriver FROM pkgs
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
		var deriver *string

		err = rows.Scan(&path, &deriver)
		if err != nil {
			return results, fmt.Errorf("error scanning rows: %w", err)
		}

		results = append(results, PathInfo{
			Path:    path,
			Deriver: deriver,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error scanning rows: %w", err)
	}

	g, _ := getDrvAttrs(results)
	fillPnameVersion(results, g)

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
// usually calculated from the transitive closures and the sets
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

	type versionSet struct {
		versions map[string]struct{}
	}

	oldPackageVersions := make(map[string]*versionSet)
	for _, path := range oldPaths {
		if _, exists := oldPackageVersions[path.Name]; !exists {
			oldPackageVersions[path.Name] = &versionSet{versions: make(map[string]struct{})}
		}
		oldPackageVersions[path.Name].versions[path.Version] = struct{}{}
	}

	newPackageVersions := make(map[string]*versionSet)
	for _, path := range newPaths {
		if _, exists := newPackageVersions[path.Name]; !exists {
			newPackageVersions[path.Name] = &versionSet{versions: make(map[string]struct{})}
		}
		newPackageVersions[path.Name].versions[path.Version] = struct{}{}
	}

	diffs := make([]PathDiff, 0)

	for name, oldVS := range oldPackageVersions {
		if newVS, exists := newPackageVersions[name]; exists {
			oldVersions := make([]string, 0, len(oldVS.versions))
			for v := range oldVS.versions {
				oldVersions = append(oldVersions, v)
			}

			newVersions := make([]string, 0, len(newVS.versions))
			for v := range newVS.versions {
				newVersions = append(newVersions, v)
			}

			oldVersionsSet := make(map[string]struct{})
			for _, v := range oldVersions {
				oldVersionsSet[v] = struct{}{}
			}

			newVersionsSet := make(map[string]struct{})
			for _, v := range newVersions {
				newVersionsSet[v] = struct{}{}
			}

			commonVersions := make(map[string]struct{})
			for v := range oldVersionsSet {
				if _, exists := newVersionsSet[v]; exists {
					commonVersions[v] = struct{}{}
				}
			}

			uniqueOldVersions := make([]string, 0)
			for v := range oldVersionsSet {
				if _, exists := newVersionsSet[v]; !exists {
					uniqueOldVersions = append(uniqueOldVersions, v)
				}
			}

			uniqueNewVersions := make([]string, 0)
			for v := range newVersionsSet {
				if _, exists := oldVersionsSet[v]; !exists {
					uniqueNewVersions = append(uniqueNewVersions, v)
				}
			}

			if len(uniqueOldVersions) == 0 && len(uniqueNewVersions) == 0 {
				continue
			}

			changed := false
			if len(uniqueOldVersions) > 0 && len(uniqueNewVersions) > 0 {
				changed = true
			}

			diff := PathDiff{
				Name: name,
			}

			if len(uniqueOldVersions) > 0 && len(uniqueNewVersions) == 0 {
				diff.Change = ChangeTypeRemove
				diff.Old = uniqueOldVersions
			} else if len(uniqueOldVersions) == 0 && len(uniqueNewVersions) > 0 {
				diff.Change = ChangeTypeAdd
				diff.New = uniqueNewVersions
			} else if changed {
				diff.Change = ChangeTypeChange
				diff.Old = uniqueOldVersions
				diff.New = uniqueNewVersions
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
		} else {
			oldVersions := make([]string, 0, len(oldVS.versions))
			for v := range oldVS.versions {
				oldVersions = append(oldVersions, v)
			}

			diff := PathDiff{
				Name:             name,
				Old:              oldVersions,
				New:              []string{},
				Change:           ChangeTypeRemove,
				SystemPathStatus: SystemPathStatusOldOnly,
			}
			diffs = append(diffs, diff)
		}
	}

	for name, newVS := range newPackageVersions {
		if _, exists := oldPackageVersions[name]; !exists {
			newVersions := make([]string, 0, len(newVS.versions))
			for v := range newVS.versions {
				newVersions = append(newVersions, v)
			}

			diff := PathDiff{
				Name:             name,
				Old:              []string{},
				New:              newVersions,
				Change:           ChangeTypeAdd,
				SystemPathStatus: SystemPathStatusNewOnly,
			}
			diffs = append(diffs, diff)
		}
	}

	return diffs
}

func formatSize(size uint64) string {
	const (
		kb = 1024
		mb = kb * 1024
		gb = mb * 1024
	)

	if size >= gb {
		return fmt.Sprintf("%.2f GiB", float64(size)/float64(gb))
	}
	if size >= mb {
		return fmt.Sprintf("%.2f MiB", float64(size)/float64(mb))
	}
	return fmt.Sprintf("%d KiB", size/kb)
}

func formatVersionList(versions []string) string {
	if len(versions) == 0 {
		return "∅"
	}
	return strings.Join(versions, ", ")
}

func displayDiffResults(closureDiff *ClosureDiff) {
	fmt.Println("Closure Comparison:")
	fmt.Println(strings.Repeat("=", 19))

	added := 0
	removed := 0
	changed := 0

	for _, diff := range closureDiff.Diffs {
		switch diff.Change {
		case ChangeTypeAdd:
			added++
		case ChangeTypeRemove:
			removed++
		case ChangeTypeChange:
			changed++
		}
	}

	fmt.Printf("Added:     %d package(s)\n", added)
	fmt.Printf("Removed:   %d package(s)\n", removed)
	fmt.Printf("Changed:   %d package(s)\n", changed)

	fmt.Println()

	addedDiffs := make([]PathDiff, 0, len(closureDiff.Diffs))
	changedDiffs := make([]PathDiff, 0, len(closureDiff.Diffs))
	removedDiffs := make([]PathDiff, 0, len(closureDiff.Diffs))

	for _, diff := range closureDiff.Diffs {
		switch diff.Change {
		case ChangeTypeAdd:
			addedDiffs = append(addedDiffs, diff)
		case ChangeTypeChange:
			changedDiffs = append(changedDiffs, diff)
		case ChangeTypeRemove:
			removedDiffs = append(removedDiffs, diff)
		}
	}

	sortPathDiffs(addedDiffs)
	sortPathDiffs(changedDiffs)
	sortPathDiffs(removedDiffs)

	if len(addedDiffs) > 0 {
		fmt.Println("Added Packages:")
		fmt.Println(strings.Repeat("=", 12))
		for _, diff := range addedDiffs {
			oldVersionStr := formatVersionList(diff.Old)
			newVersionStr := formatVersionList(diff.New)
			systemPathIndicator := " "
			switch diff.SystemPathStatus {
			case SystemPathStatusNewOnly, SystemPathStatusBoth:
				systemPathIndicator = "x"
			}
			fmt.Printf("[%s] %s: %s -> %s\n", systemPathIndicator, diff.Name, oldVersionStr, newVersionStr)
		}
		fmt.Println()
	}

	if len(removedDiffs) > 0 {
		fmt.Println("Removed Packages:")
		fmt.Println(strings.Repeat("=", 14))
		for _, diff := range removedDiffs {
			oldVersionStr := formatVersionList(diff.Old)
			newVersionStr := formatVersionList(diff.New)
			systemPathIndicator := " "
			switch diff.SystemPathStatus {
			case SystemPathStatusOldOnly, SystemPathStatusBoth:
				systemPathIndicator = "*"
			}
			fmt.Printf("[%s] %s: %s -> %s\n", systemPathIndicator, diff.Name, oldVersionStr, newVersionStr)
		}
		fmt.Println()
	}

	if len(changedDiffs) > 0 {
		fmt.Println("Changed Packages:")
		fmt.Println(strings.Repeat("=", 15))
		for _, diff := range changedDiffs {
			oldVersionStr := formatVersionList(diff.Old)
			newVersionStr := formatVersionList(diff.New)
			systemPathIndicator := " "
			switch diff.SystemPathStatus {
			case SystemPathStatusBoth:
				systemPathIndicator = "x"
			case SystemPathStatusNewOnly:
				systemPathIndicator = "↑"
			case SystemPathStatusOldOnly:
				systemPathIndicator = "↓"
			}
			fmt.Printf("[%s] %s: %s -> %s\n", systemPathIndicator, diff.Name, oldVersionStr, newVersionStr)
		}
		fmt.Println()
	}

	fmt.Println("Closure Sizes:")
	fmt.Println(strings.Repeat("=", 14))

	oldSizeStr := formatSize(closureDiff.OldSize)
	newSizeStr := formatSize(closureDiff.NewSize)

	if closureDiff.OldSize == closureDiff.NewSize {
		fmt.Printf("Old: %s\n", oldSizeStr)
		fmt.Printf("New: %s\n", newSizeStr)
	} else {
		if closureDiff.NewSize > closureDiff.OldSize {
			change := closureDiff.NewSize - closureDiff.OldSize
			changeStr := formatSize(change)
			fmt.Printf("Old: %s\n", oldSizeStr)
			fmt.Printf("New: %s (+%s)\n", newSizeStr, changeStr)
		} else {
			change := closureDiff.OldSize - closureDiff.NewSize
			changeStr := formatSize(change)
			fmt.Printf("Old: %s (-%s)\n", oldSizeStr, changeStr)
			fmt.Printf("New: %s\n", newSizeStr)
		}
	}
}

func sortPathDiffs(diffs []PathDiff) {
	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Name < diffs[j].Name
	})
}

type drvAttrs struct {
	Drvs map[string]struct {
		Env             map[string]string `json:"env"`
		StructuredAttrs map[string]any    `json:"structuredAttrs,omitempty"`
	} `json:"derivations"`
}

// Calculate the dependency graph of a Nix store derivation
// using `nix derivation show`.
//
// This is used for determining what the pname and version are
// exactly, if available, using `nix derivation show`.
func getDrvAttrs(paths []PathInfo) (*drvAttrs, error) {
	cmd := exec.Command("nix", "derivation", "show", "--stdin")

	var drvArgs bytes.Buffer
	for _, p := range paths {
		if p.Deriver == nil {
			continue
		}

		drv := *p.Deriver
		if _, err := os.Stat(drv); err != nil {
			continue
		}

		drvArgs.WriteString(*p.Deriver)
		drvArgs.WriteString("\n")
	}

	cmd.Stdin = bytes.NewReader(drvArgs.Bytes())

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	var graph drvAttrs
	if err := json.Unmarshal(stdout.Bytes(), &graph); err != nil {
		return nil, err
	}

	return &graph, nil
}

func fillPnameVersion(paths []PathInfo, graph *drvAttrs) {
	for i, path := range paths {
		if graph == nil || path.Deriver == nil {
			paths[i].Name, paths[i].Version = parsePnameAndVersion(path.Path)
			continue
		}

		name := paths[i].Name
		version := paths[i].Version

		// Helper: only set if empty
		setName := func(v string) {
			if name == "" && v != "" {
				name = v
			}
		}
		setVersion := func(v string) {
			if version == "" && v != "" {
				version = v
			}
		}

		// Derivers in the graph are addressed by basename rather
		// than by the whole store path.
		baseDrvPath := filepath.Base(*path.Deriver)
		drv, drvExists := graph.Drvs[baseDrvPath]
		if !drvExists {
			paths[i].Name, paths[i].Version = parsePnameAndVersion(path.Path)
			continue
		}

		// Use structuredAttrs first before env.
		if attrs := drv.StructuredAttrs; attrs != nil {
			if pnameAttr, exists := attrs["pname"].(string); exists {
				setName(pnameAttr)
			} else if nameAttr, exists2 := attrs["name"].(string); exists2 {
				setName(nameAttr)
			}

			if versionAttr, exists := attrs["version"].(string); exists {
				setVersion(versionAttr)
			}
		}

		// Then, check if the attr is in env.
		if attrs := drv.Env; attrs != nil {
			if pnameAttr, exists := attrs["pname"]; exists {
				setName(pnameAttr)
			} else if nameAttr, exists2 := attrs["name"]; exists2 {
				setName(nameAttr)
			}

			if versionAttr, exists := attrs["version"]; exists {
				setVersion(versionAttr)
			}
		}

		if paths[i].Name == "" || paths[i].Version == "" {
			n, v := parsePnameAndVersion(path.Path)
			if paths[i].Name == "" {
				setName(n)
			}
			if paths[i].Version == "" {
				setVersion(v)
			}
		}

		paths[i].Name = name
		paths[i].Version = version
	}
}
