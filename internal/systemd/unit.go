package systemdUtils

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/coreos/go-systemd/v22/unit"
)

type UnitInfo map[string]map[string][]string

// Parse a systemd unit file into a UnitInfo type.
//
// If a directory with the same basename ending in .d
// exists next to the unit file, it will be assumed to
// contain override files which will be parsed as well
// and handled properly.
func ParseUnit(unitFilePath string, baseUnitFilePath string) (UnitInfo, error) {
	unitData := make(UnitInfo)

	parseAndMerge := func(path string) error {
		file, err := os.Open(path)
		if err != nil {
			return fmt.Errorf("failed to open %s: %s", path, err)
		}
		defer func() { _ = file.Close() }()

		sections, err := unit.DeserializeSections(file)
		if err != nil {
			return fmt.Errorf("failed to parse unit file %s: %s", path, err)
		}

		unitData.merge(sections)
		return nil
	}

	if err := parseAndMerge(baseUnitFilePath); err != nil {
		return nil, err
	}

	matches, _ := filepath.Glob(fmt.Sprintf("%s.d/*.conf", baseUnitFilePath))
	for _, path := range matches {
		_ = parseAndMerge(path)
	}

	if unitFilePath != baseUnitFilePath {
		if _, err := os.Stat(unitFilePath); err == nil {
			matches, _ := filepath.Glob(fmt.Sprintf("%s.d/*.conf", unitFilePath))
			for _, path := range matches {
				_ = parseAndMerge(path)
			}
		}
	}

	return unitData, nil
}

// Retrieve a single property from a UnitInfo type.
//
// If the property is multi-valued, the last value is returned.
// Consider using
//
// A return value of nil means the property does not exist.
func (i UnitInfo) GetProperty(section string, property string) *string {
	sec, ok := i[section]
	if !ok {
		return nil
	}

	values, ok := sec[property]
	if !ok {
		return nil
	}

	if len(values) == 0 {
		return nil
	}

	return &values[len(values)-1]
}

// Retrieve a multi-value property from a UnitInfo type.
//
// A return value of nil means the property does not exist.
func (i UnitInfo) GetPropertyMulti(section string, property string) []string {
	sec, ok := i[section]
	if !ok {
		return nil
	}

	values, ok := sec[property]
	if !ok {
		return nil
	}

	return values
}

func (i UnitInfo) GetBoolean(section string, property string, defaultValue bool) bool {
	val := i.GetProperty(section, property)
	if val == nil {
		return defaultValue
	}

	return ParseBool(*val)
}

func (i UnitInfo) merge(sections []*unit.UnitSection) {
	for _, sec := range sections {
		if _, ok := i[sec.Section]; !ok {
			i[sec.Section] = make(map[string][]string)
		}

		for _, entry := range sec.Entries {
			i[sec.Section][entry.Name] = append(
				i[sec.Section][entry.Name],
				entry.Value,
			)
		}
	}
}

// Parse a systemd boolean value string into a Go boolean.
func ParseBool(value string) bool {
	switch value {
	case "1", "yes", "true", "on":
		return true
	default:
		return false
	}
}

type UnitComparison int

const (
	UnitComparisonEqual UnitComparison = iota
	UnitComparisonNeedsRestart
	UnitComparisonNeedsReload
)

var unitSectionIgnores = map[string]struct{}{
	"X-Reload-Triggers": {},
	"Description":       {},
	"Documentation":     {},
	"OnFailure":         {},
	"OnSuccess":         {},
	"IgnoreOnIsolate":   {},
	"OnFailureJobMode":  {},
	"StopWhenUnneeded":  {},
	"RefuseManualStart": {},
	"RefuseManualStop":  {},
	"AllowIsolate":      {},
	"CollectMode":       {},
	"SourcePath":        {},
}

func CompareUnits(currentUnit UnitInfo, newUnit UnitInfo) UnitComparison {
	result := UnitComparisonEqual

	sectionsToCompare := make(map[string]struct{}, len(newUnit))
	for key := range newUnit {
		sectionsToCompare[key] = struct{}{}
	}

	for sectionName, section := range currentUnit {
		if _, ok := sectionsToCompare[sectionName]; !ok {
			if sectionName == "Unit" {
				for iniKey := range section {
					if _, ignore := unitSectionIgnores[iniKey]; !ignore {
						return UnitComparisonNeedsRestart
					}
				}

				continue
			} else {
				return UnitComparisonNeedsRestart
			}
		}

		delete(sectionsToCompare, sectionName)

		iniKeysToCompare := make(map[string]struct{}, len(section))
		for key := range section {
			iniKeysToCompare[key] = struct{}{}
		}

		for iniKey, currentValue := range section {
			delete(iniKeysToCompare, iniKey)

			newValue := newUnit.GetPropertyMulti(sectionName, iniKey)
			if newValue == nil {
				if sectionName == "Unit" {
					if _, ignore := unitSectionIgnores[iniKey]; ignore {
						continue
					}
				}

				return UnitComparisonNeedsRestart
			}

			if !slices.Equal(currentValue, newValue) {
				if sectionName == "Unit" {
					if iniKey == "X-Reload-Triggers" {
						result = UnitComparisonNeedsReload
						continue
					} else if _, ignore := unitSectionIgnores[iniKey]; ignore {
						continue
					}
				}

				// If this is a mount unit, changes to `Options` can be ignored.
				if sectionName == "Mount" && iniKey == "Options" {
					result = UnitComparisonNeedsReload
					continue
				}

				return UnitComparisonNeedsRestart
			}
		}

		if len(iniKeysToCompare) > 0 {
			if sectionName == "Unit" {
				for iniKey := range iniKeysToCompare {
					if iniKey == "X-Reload-Triggers" {
						result = UnitComparisonNeedsReload
					} else if _, ignore := unitSectionIgnores[iniKey]; ignore {
						continue
					} else {
						return UnitComparisonNeedsRestart
					}
				}
			} else {
				return UnitComparisonNeedsRestart
			}
		}
	}

	remainingSections := len(sectionsToCompare)

	if remainingSections > 0 {
		if remainingSections == 1 {
			unitSection, exists := newUnit["Unit"]
			if !exists {
				return UnitComparisonNeedsRestart
			}

			for iniKey := range unitSection {
				if _, ignore := unitSectionIgnores[iniKey]; !ignore {
					return UnitComparisonNeedsRestart
				} else if iniKey == "X-Reload-Triggers" {
					result = UnitComparisonNeedsReload
				}
			}
		} else {
			return UnitComparisonNeedsRestart
		}
	}

	return result
}
