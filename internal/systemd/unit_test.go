package systemdUtils

import (
	"testing"
)

func TestCompareUnits(t *testing.T) {
	tests := []struct {
		name     string
		current  UnitInfo
		new      UnitInfo
		expected UnitComparison
	}{
		{
			name: "equal units",
			current: UnitInfo{
				"Unit": {
					"Description": {"Test service"},
				},
			},
			new: UnitInfo{
				"Unit": {
					"Description": {"Test service"},
				},
			},
			expected: UnitComparisonEqual,
		},
		{
			name: "changed unit key requires restart",
			current: UnitInfo{
				"Service": {
					"ExecStart": {"/bin/old"},
				},
			},
			new: UnitInfo{
				"Service": {
					"ExecStart": {"/bin/new"},
				},
			},
			expected: UnitComparisonNeedsRestart,
		},
		{
			name: "reload trigger only requires reload",
			current: UnitInfo{
				"Unit": {
					"X-Reload-Triggers": {"foo"},
				},
			},
			new: UnitInfo{
				"Unit": {
					"X-Reload-Triggers": {"bar"},
				},
			},
			expected: UnitComparisonNeedsReload,
		},
		{
			name: "ignored unit key change does not restart",
			current: UnitInfo{
				"Unit": {
					"Description": {"Old"},
				},
			},
			new: UnitInfo{
				"Unit": {
					"Description": {"New"},
				},
			},
			expected: UnitComparisonEqual, // ignored key
		},
		{
			name: "mount options change only reloads",
			current: UnitInfo{
				"Mount": {
					"Options": {"old", "opt"},
				},
			},
			new: UnitInfo{
				"Mount": {
					"Options": {"new", "opt"},
				},
			},
			expected: UnitComparisonNeedsReload,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CompareUnits(tt.current, tt.new)
			if got != tt.expected {
				t.Errorf("compareUnits() = %v, want %v", got, tt.expected)
			}
		})
	}
}
