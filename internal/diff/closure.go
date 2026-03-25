package diff

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Retrieve ALL the transitive dependencies of a closure in the Nix store,
// as well as the total size of the closure.
//
// This includes all paths, so if a caller wants to differentiate between
// user-added packages and intermediate derivations such as Nix store
// paths, they should use `getSystemPathDrvPaths()` to obtain these more
// explicit paths.
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
			return nil, fmt.Errorf("error scanning rows: %w", err)
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
			return nil, fmt.Errorf("error scanning rows: %w", err)
		}

		results = append(results, PathInfo{
			Path:    path,
			Deriver: deriver,
		})
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error scanning rows: %w", err)
	}

	return results, nil
}

type drvAttrs struct {
	Drvs map[string]struct {
		Env             map[string]string `json:"env"`
		StructuredAttrs map[string]any    `json:"structuredAttrs,omitempty"`
	} `json:"derivations"`
}

// Get the deriver information for a list of Nix store
// derivations using `nix derivation show`.
//
// This is used for determining what the pname and version are
// exactly, if their derivers are available.
func getDrvAttrs(pathLists ...[]PathInfo) (*drvAttrs, error) {
	cmd := exec.Command("nix", "derivation", "show", "--stdin")

	var drvArgs bytes.Buffer
	for _, paths := range pathLists {
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
	}

	cmd.Stdin = bytes.NewReader(drvArgs.Bytes())

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd.Stderr = &stderr
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("nix derivation show failed: %w\nstderr: %s", err, stderr.String())
	}

	var graph drvAttrs
	if err := json.Unmarshal(stdout.Bytes(), &graph); err != nil {
		return nil, err
	}

	return &graph, nil
}

// Fill in the Name and Version attributes for a slice of
// store paths. This can either be done using the drvAttrs
// map if it was calculated from `nix derivation show` for
// more accurate versions, or using pname-version parsing
// if not found in the map or if it is nil
func fillPnameVersion(paths []PathInfo, drvAttrMap *drvAttrs) {
	for i, path := range paths {
		if drvAttrMap == nil || path.Deriver == nil {
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
		drv, drvExists := drvAttrMap.Drvs[baseDrvPath]
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

		if name == "" || version == "" {
			n, v := parsePnameAndVersion(path.Path)
			setName(n)
			setVersion(v)
		}

		// Strip the version in case it still is attached to the pname
		// according to the pname-version splitting rules. This is
		// probably super uncommon, but worth checking for anyway in
		// case someone uses git commits as version strings and they
		// start with a letter.
		if before, ok := strings.CutSuffix(name, version); ok {
			name = before
			name = strings.TrimSuffix(name, "-")
		}

		paths[i].Name = name
		paths[i].Version = version
	}
}
