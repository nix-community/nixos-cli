package diff

import (
	"database/sql"
	"fmt"
	"sort"

	_ "modernc.org/sqlite"

	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/nix-community/nixos-cli/internal/utils/set"
)

type DiffCommandOptions struct {
	DiffTool    settings.DiffTool
	DiffToolCmd []string
}

type ClosureDiff struct {
	Old   ClosureDiffPathInfo
	New   ClosureDiffPathInfo
	Diffs []PathDiff
}

type ClosureDiffPathInfo struct {
	Path string
	Size uint64
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

type Closure struct {
	Size  uint64
	Paths []PathInfo
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
		if len(opts.DiffToolCmd) == 0 {
			log.Warn("differ.command is empty")
			tool = settings.DifferNix
			break
		}
		cmd := opts.DiffToolCmd[0]
		if !s.HasCommand(cmd) {
			log.Warnf("differ.command uses '%s', but `%s` is not executable", cmd, cmd)
			tool = settings.DifferNix
		}
	case settings.DifferNix:
	default:
		return fmt.Errorf("invalid differ type '%v'", tool)
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
	}

	s.Logger().CmdArray(argv)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	_, err := s.Run(cmd)
	return err
}

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
		Old: ClosureDiffPathInfo{
			Path: before,
			Size: closuresBefore.Size,
		},
		New: ClosureDiffPathInfo{
			Path: after,
			Size: closuresAfter.Size,
		},
		Diffs: diffs,
	}, nil
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
	oldSystemPathSet := set.New[string]()
	for _, p := range oldSystemPath {
		oldSystemPathSet.Add(p.Name)
	}

	newSystemPathSet := set.New[string]()
	for _, path := range newSystemPath {
		newSystemPathSet.Add(path.Name)
	}

	oldPackageVersions := buildPackageVersions(oldPaths)
	newPackageVersions := buildPackageVersions(newPaths)

	diffs := make([]PathDiff, 0)

	for name, oldVersions := range oldPackageVersions {
		if newVersions, exists := newPackageVersions[name]; exists {
			uniqueOldVersions := oldVersions.Difference(newVersions).Slice()
			uniqueNewVersions := newVersions.Difference(oldVersions).Slice()

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
			var status SystemPathStatus
			if _, ok := oldSystemPathSet[name]; ok {
				status = SystemPathStatusOldOnly
			} else {
				status = SystemPathStatusNeither
			}
			diff := PathDiff{
				Name:             name,
				Old:              oldVersions.Slice(),
				New:              []string{},
				Change:           ChangeTypeRemove,
				SystemPathStatus: status,
			}
			diffs = append(diffs, diff)
		}
	}

	for name, newVersions := range newPackageVersions {
		if _, exists := oldPackageVersions[name]; !exists {
			var status SystemPathStatus
			if _, ok := newSystemPathSet[name]; ok {
				status = SystemPathStatusNewOnly
			} else {
				status = SystemPathStatusNeither
			}
			diff := PathDiff{
				Name:             name,
				Old:              []string{},
				New:              newVersions.Slice(),
				Change:           ChangeTypeAdd,
				SystemPathStatus: status,
			}
			diffs = append(diffs, diff)
		}
	}

	sort.Slice(diffs, func(i, j int) bool {
		return diffs[i].Name < diffs[j].Name
	})

	return diffs
}
