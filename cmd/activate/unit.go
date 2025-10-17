//go:build linux

package activate

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	systemdDbus "github.com/coreos/go-systemd/v22/dbus"
	systemdUnit "github.com/coreos/go-systemd/v22/unit"
	systemdUtils "github.com/nix-community/nixos-cli/internal/systemd"
)

const (
	START_LIST_FILE   = "/run/nixos/start-list"
	RESTART_LIST_FILE = "/run/nixos/restart-list"
	RELOAD_LIST_FILE  = "/run/nixos/reload-list"
)

var (
	templateUnitPattern = regexp.MustCompile(`^(.*)@([^\.]*)\.(.*)$`)
	unitNamePattern     = regexp.MustCompile(`^(.*)\.[[:lower:]]*$`)
)

type UnitState struct {
	State    string
	SubState string
}

type UnitList map[string]struct{}

func (l UnitList) Add(unit string) {
	l[unit] = struct{}{}
}

func (l UnitList) Has(unit string) bool {
	_, ok := l[unit]
	return ok
}

func (l UnitList) Remove(unit string) {
	delete(l, unit)
}

type UnitLists struct {
	toplevel string

	Start   UnitList
	Stop    UnitList
	Restart UnitList
	Reload  UnitList
	Skip    UnitList
	Filter  UnitList
}

func makeUnitLists(toplevel string) *UnitLists {
	unitsToStart := readUnitsListFile(START_LIST_FILE)
	unitsToRestart := readUnitsListFile(RESTART_LIST_FILE)
	unitsToReload := readUnitsListFile(RELOAD_LIST_FILE)

	unitsToStop := make(UnitList)
	unitsToSkip := make(UnitList)
	unitsToFilter := make(UnitList)

	return &UnitLists{
		toplevel: toplevel,

		Start:   unitsToStart,
		Stop:    unitsToStop,
		Restart: unitsToRestart,
		Reload:  unitsToReload,
		Skip:    unitsToSkip,
		Filter:  unitsToFilter,
	}
}

func (l *UnitLists) ClassifyActiveUnits(ctx context.Context, units map[string]UnitState) error {
	for unit, state := range units {
		resolvedUnit := resolveUnit(unit, l.toplevel)

		// Only handle resolved units that actually exist and are in an
		// activated/activating state.
		_, err := os.Stat(resolvedUnit.CurrentBaseFile)
		if err != nil || (state.State != "active" && state.State != "activating") {
			continue
		}

		path, err := filepath.EvalSymlinks(resolvedUnit.NewBaseFile)
		treatAsNullUnit := err != nil || path == "/dev/null"

		if treatAsNullUnit {
			// Masked units (aka units that are symlinked to /dev/null) should be stopped
			// if they contain the X-StopOnRemoval attribute.
			unitInfo, err := systemdUtils.ParseUnit(resolvedUnit.CurrentUnitFile, resolvedUnit.CurrentBaseFile)
			if err != nil {
				return err
			}

			if unitInfo.GetBoolean("Unit", "X-StopOnRemoval", false) {
				l.Stop.Add(unit)
			}
		} else if strings.HasSuffix(unit, ".target") {
			newUnitInfo, err := systemdUtils.ParseUnit(resolvedUnit.NewUnitFile, resolvedUnit.NewBaseFile)
			if err != nil {
				return err
			}

			// FIXME: The suspend target is sometimes active after the system has
			// resumed, which should probably not be the case. Ignore for now.
			skipStart := unit == "suspend.target" ||
				unit == "hibernate.target" ||
				unit == "hybrid-sleep.target" ||
				newUnitInfo.GetBoolean("Unit", "RefuseManualStart", false) ||
				newUnitInfo.GetBoolean("Unit", "X-OnlyManualStart", false)

			// Restart all active target units. This should start most
			// changed units we stop here as well as any new dependencies
			// (including new mounts and swap devices).
			if !skipStart {
				l.Start.Add(unit)
				recordUnit(START_LIST_FILE, unit)
				if os.Getenv("STC_DISPLAY_ALL_UNITS") != "1" {
					l.Filter.Add(unit)
				}
			}

			// Stop targets that have X-StopOnReconfiguration set. This is necessary to respect
			// dependency orderings involving targets: if unit X starts after target Y and
			// target Y starts after unit Z, then if X and Z have both changed, then X should
			// be restarted after Z. However, if target Y is in the "active" state, X and Z
			// will be restarted at the same time because X's dependency on Y is already
			// satisfied. Thus, we need to stop Y first. Stopping a target generally has no
			// effect on other units (unless there is a PartOf dependency), so this is just a
			// bookkeeping thing to get systemd to do the right thing.
			if newUnitInfo.GetBoolean("Unit", "X-StopOnReconfiguration", false) {
				l.Stop.Add(unit)
			}
		} else {
			currentUnitInfo, err := systemdUtils.ParseUnit(resolvedUnit.CurrentUnitFile, resolvedUnit.CurrentBaseFile)
			if err != nil {
				return err
			}

			newUnitInfo, err := systemdUtils.ParseUnit(resolvedUnit.NewUnitFile, resolvedUnit.NewBaseFile)
			if err != nil {
				return err
			}

			switch systemdUtils.CompareUnits(currentUnitInfo, newUnitInfo) {
			case systemdUtils.UnitComparisonNeedsRestart:
				err := l.ClassifyModifiedUnit(
					unit,
					resolvedUnit.BaseName,
					resolvedUnit.NewUnitFile,
					resolvedUnit.NewBaseFile,
					newUnitInfo,
					units,
				)
				if err != nil {
					return err
				}
			case systemdUtils.UnitComparisonNeedsReload:
				if !l.Restart.Has(unit) {
					l.Reload.Add(unit)
					recordUnit(RELOAD_LIST_FILE, unit)
				}
			}
		}
	}

	return nil
}

func (l UnitList) Filter(unitsToFilter UnitList) UnitList {
	newList := make(UnitList)

	for unit := range l {
		if !unitsToFilter.Has(unit) {
			newList.Add(unit)
		}
	}

	return newList
}

func (l UnitList) Sorted() []string {
	keys := make([]string, 0, len(l))
	for unit := range l {
		keys = append(keys, strings.ToLower(unit))
	}

	sort.Strings(keys)
	return keys
}

func (l *UnitLists) ClassifyModifiedUnit(
	unit string,
	baseName string,
	newUnitFile string,
	newBaseUnitFile string,
	newUnitInfo systemdUtils.UnitInfo,
	activeUnits map[string]UnitState,
) error {
	// If the new unit info is not passed, then we are running after the
	// activation script has been executed. This means that services that
	// require stopping have already been stopped before this point.
	//
	// As such, for these units, just use the restart mechanism, instead
	// of adding it to the stop/start unit lists just for it to be
	// silently ignored.
	useRestartToStopStartUnit := newUnitInfo == nil

	// These units cannot be restarted directly, so do nothing.
	//
	// Slices and paths don't have to be restarted since properties
	// (resource limits, inotify watches) get applied on daemon-reload.
	if unit == "sysinit.target" ||
		unit == "basic.target" ||
		unit == "multi-user.target" ||
		unit == "graphical.target" ||
		strings.HasSuffix(unit, ".path") ||
		strings.HasSuffix(unit, ".slice") {
		return nil
	}

	// FIXME: do something?
	// Attempt to fix this: https://github.com/NixOS/nixpkgs/pull/141192
	// Revert of the attempt: https://github.com/NixOS/nixpkgs/pull/147609
	// More details: https://github.com/NixOS/nixpkgs/issues/74899#issuecomment-981142430
	if strings.HasSuffix(unit, ".socket") {
		return nil
	}

	// Just restart mount units. We wouldn't have gotten into this condition if only `Options`
	// was changed, in which case the unit would be reloaded. The only exception is / and /nix
	// because it's very unlikely we can safely unmount them so we reload them instead. This
	// means that we may not get all changes into the running system but it's better than
	// crashing it.
	if strings.HasSuffix(unit, ".mount") {
		if unit == "-.mount" || unit == "nix.mount" {
			l.Reload.Add(unit)
			recordUnit(RELOAD_LIST_FILE, unit)
		} else {
			l.Restart.Add(unit)
			recordUnit(RESTART_LIST_FILE, unit)
		}

		return nil
	}

	if newUnitInfo == nil {
		info, err := systemdUtils.ParseUnit(newUnitFile, newBaseUnitFile)
		if err != nil {
			return err
		}
		newUnitInfo = info
	}

	if newUnitInfo.GetBoolean("Service", "X-ReloadIfChanged", false) && !l.Restart.Has(unit) {
		var reloadUnit bool
		if useRestartToStopStartUnit {
			reloadUnit = !l.Restart.Has(unit)
		} else {
			reloadUnit = !l.Stop.Has(unit)
		}

		if reloadUnit {
			l.Reload.Add(unit)
			recordUnit(RELOAD_LIST_FILE, unit)
		}

		return nil
	}

	if !newUnitInfo.GetBoolean("Service", "X-RestartIfChanged", true) ||
		newUnitInfo.GetBoolean("Unit", "RefuseManualStop", false) ||
		newUnitInfo.GetBoolean("Unit", "X-OnlyManualStart", false) {
		l.Skip.Add(unit)
		return nil
	}

	// It doesn't make sense to stop and start non-services because they can't have
	// the `ExecStop` property.
	if !newUnitInfo.GetBoolean("Service", "X-StopIfChanged", true) || !strings.HasSuffix(unit, ".service") {
		l.Restart.Add(unit)
		recordUnit(RESTART_LIST_FILE, unit)

		if l.Reload.Has(unit) {
			l.Reload.Remove(unit)
			unrecordUnit(RELOAD_LIST_FILE, unit)
		}

		return nil
	}

	// If this unit is socket-activated, then stop the socket unit(s) as well, and
	// restart the socket(s) instead of the service.
	//
	// We count as "socket-activated" any unit that doesn't declare itself not so
	// via X-NotSocketActivated, that has any associated .socket units.
	socketActivated := false

	if strings.HasSuffix(unit, ".service") {
		var sockets []string
		if val := newUnitInfo.GetProperty("Service", "Sockets"); val != nil {
			sockets = strings.Fields(*val)
		}

		if len(sockets) == 0 {
			sockets = append(sockets, fmt.Sprintf("%s.socket", baseName))
		}

		for _, socket := range sockets {
			if _, ok := activeUnits[socket]; !ok {
				continue
			}

			if useRestartToStopStartUnit {
				l.Restart.Add(socket)
			} else {
				l.Stop.Add(socket)
			}

			socketUnitPath := filepath.Join(l.toplevel, "etc/systemd/system", socket)
			if _, err := os.Stat(socketUnitPath); err == nil {
				if useRestartToStopStartUnit {
					l.Restart.Add(socket)
					recordUnit(RESTART_LIST_FILE, socket)
				} else {
					l.Start.Add(socket)
					recordUnit(START_LIST_FILE, socket)
				}

				socketActivated = true
			}

			if l.Reload.Has(unit) {
				l.Reload.Remove(unit)
				unrecordUnit(RELOAD_LIST_FILE, unit)
			}
		}
	}

	if newUnitInfo.GetBoolean("Service", "X-NotSocketActivated", false) {
		// If the unit explicitly opts out of socket
		// activation, restart it as if it weren't (but do
		// restart its sockets, that's fine):
		socketActivated = false
	}

	// If the unit is not socket-activated, record that this unit
	// needs to be started below.
	if !socketActivated {
		if useRestartToStopStartUnit {
			l.Restart.Add(unit)
			recordUnit(RESTART_LIST_FILE, unit)
		} else {
			l.Start.Add(unit)
			recordUnit(START_LIST_FILE, unit)
		}

		// And then remove it from reload if need be,
		// to avoid restarting/starting and reloading
		// a service.
		if l.Reload.Has(unit) {
			l.Reload.Remove(unit)
			unrecordUnit(RELOAD_LIST_FILE, unit)
		}
	}

	return nil
}

// Given a map of currently mounted filesystems and the new map
// of filesystems to handle, classify whether or not their units
// should be restarted, reloaded, or stopped (aka unmounted).
func (l UnitLists) ClassifyFilesystemUnits(
	currentFilesystems map[string]Filesystem,
	newFilesystems map[string]Filesystem,
) {
	for mountpoint, currentFS := range currentFilesystems {
		unit := fmt.Sprintf("%s.mount", systemdUnit.UnitNamePathEscape(mountpoint))

		newFS, stillExists := newFilesystems[mountpoint]
		if !stillExists {
			// Unmount filesystem units that have disappeared from the
			// new system.
			l.Stop.Add(unit)
			continue
		}

		if currentFS.Type != newFS.Type || currentFS.Device != newFS.Device {
			// "/" and "/nix" mountpoints should never be restarted,
			// or these could cause a system crash.
			//
			// Only reload these special mountpoints if their
			// mount options have changed.
			if mountpoint == "/" || mountpoint == "/nix" {
				if currentFS.Options != newFS.Options {
					l.Reload.Add(unit)
					recordUnit(RELOAD_LIST_FILE, unit)
				} else {
					l.Skip.Add(unit)
				}
			} else {
				// Device and type changes require unmounting the existing
				// filesystem, which can only be achieved with a unit restart.
				l.Restart.Add(unit)
				recordUnit(RESTART_LIST_FILE, unit)
			}
		} else if currentFS.Options != newFS.Options {
			// Any units reloaded here will respect the soft "remount"
			// option, so there's no need to handle this specially.
			l.Reload.Add(unit)
			recordUnit(RELOAD_LIST_FILE, unit)
		}
	}
}

// Ask the currently running systemd instance via dbus which units are active.
//
// Returns a map where the key is the unit name and the value is the unit's
// state and substate.
func getActiveUnits(ctx context.Context, systemd *systemdDbus.Conn) (map[string]UnitState, error) {
	allUnits, err := systemd.ListUnitsContext(ctx)
	if err != nil {
		return nil, err
	}

	unitMap := make(map[string]UnitState, len(allUnits))

	for _, unit := range allUnits {
		if unit.ActiveState != "inactive" && unit.Followed == "" {
			unitMap[unit.Name] = UnitState{
				State:    unit.ActiveState,
				SubState: unit.SubState,
			}
		}
	}

	return unitMap, nil
}

func unitIsActive(ctx context.Context, systemd *systemdDbus.Conn, unit string) (bool, error) {
	prop, err := systemd.GetUnitPropertyContext(ctx, unit, "ActiveState")
	if err != nil {
		return false, err
	}

	state := prop.Value.Value().(string)

	return state == "active" || state == "activating", nil
}

type ResolvedUnit struct {
	// Original unit string (foo@bar.service)
	Unit string
	// Resolved base unit (foo@.service)
	BaseUnit string
	// Base name without extension (foo)
	BaseName string
	// Location of current unit file (/etc/systemd/system/<unit>)
	CurrentUnitFile string
	// Location of new generation unit file (<toplevel>/etc/systemd/system/<unit>)
	NewUnitFile string
	// Location of current base unit if templated (/etc/systemd/system/<base_unit>)
	CurrentBaseFile string
	// Location of new base file (<toplevel>/etc/systemd/system/<base_unit>)
	NewBaseFile string
}

// Construct the paths associated with a unit.
//
// If the unit is templated, then strip the template value from the unit name
// and use that in each base value.
func resolveUnit(unit, toplevel string) ResolvedUnit {
	currentUnitFile := filepath.Join("/etc/systemd/system", unit)
	newUnitFile := filepath.Join(toplevel, "etc/systemd/system", unit)

	baseUnit := unit
	currentBaseFile := currentUnitFile
	newBaseFile := newUnitFile

	// Detect template instances and strip the template name from the base
	// units and file paths.
	if matches := templateUnitPattern.FindStringSubmatch(unit); len(matches) > 3 {
		templateName, _, unitType := matches[1], matches[2], matches[3]

		if _, err := os.Stat(currentUnitFile); os.IsNotExist(err) {
			if _, err := os.Stat(newUnitFile); os.IsNotExist(err) {
				baseUnit = fmt.Sprintf("%s@.%s", templateName, unitType)
				currentBaseFile = filepath.Join("/etc/systemd/system", baseUnit)
				newBaseFile = filepath.Join(toplevel, "etc/systemd/system", baseUnit)
			}
		}
	}

	// Extract base name (strip the extension like .service, .mount)
	baseName := baseUnit
	if matches := unitNamePattern.FindStringSubmatch(baseUnit); len(matches) > 1 {
		baseName = matches[1]
	}

	return ResolvedUnit{
		Unit:            unit,
		BaseUnit:        baseUnit,
		BaseName:        baseName,
		CurrentUnitFile: currentUnitFile,
		NewUnitFile:     newUnitFile,
		CurrentBaseFile: currentBaseFile,
		NewBaseFile:     newBaseFile,
	}
}

// Read a list of units from a unit list file into a UnitList set.
//
// Units are split by newlines, and empty lines are ignored.
func readUnitsListFile(path string) UnitList {
	units := make(UnitList)

	file, err := os.Open(path)
	if err != nil {
		return units
	}
	defer func() { _ = file.Close() }()

	s := bufio.NewScanner(file)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line != "" {
			units.Add(line)
		}
	}

	return units
}

// Whether or not to record units into files for more resilience
// if activation is terminated early.
//
// This should be disabled during dry activation.
var recordUnits = true

// Append a unit to a unit list file.
//
// If in dry activation mode, this is skipped.
func recordUnit(filename string, unit string) {
	if !recordUnits {
		return
	}

	file, err := os.OpenFile(filename, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	_, _ = file.WriteString(unit + "\n")
}

// Remove a unit from a unit list file by reading its
// contents, filtering the value out, and rewriting the
// file again.
//
// If in dry activation mode, this is skipped.
func unrecordUnit(filename string, unit string) {
	if !recordUnits {
		return
	}

	file, err := os.Open(filename)
	if err != nil {
		return
	}
	defer func() { _ = file.Close() }()

	var lines []string
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if line != unit && line != "" {
			lines = append(lines, line)
		}
	}

	_ = os.WriteFile(filename, []byte(strings.Join(lines, "\n")+"\n"), 0o644)
}
