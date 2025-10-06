package systemdUtils

import (
	"math"
	"testing"
	"time"
)

func durationsApproxEqual(d1, d2, tolerance time.Duration) bool {
	diff := d1 - d2
	return math.Abs(float64(diff)) <= float64(tolerance)
}

func TestDurationFromTimeSpan(t *testing.T) {
	const tolerance = time.Millisecond

	tests := []struct {
		span      string
		expected  time.Duration
		expectErr bool
	}{
		{"1s", time.Second, false},
		{"1second", time.Second, false},
		{"2m", 2 * time.Minute, false},
		{"2min", 2 * time.Minute, false},
		{"3h", 3 * time.Hour, false},
		{"3hours", 3 * time.Hour, false},
		{"1d", 24 * time.Hour, false},
		{"1day", 24 * time.Hour, false},
		{"1w", 7 * 24 * time.Hour, false},
		{"10weeks", 10 * 7 * 24 * time.Hour, false},
		{"1h30m", 90 * time.Minute, false},
		{"2d3h45m", 2*24*time.Hour + 3*time.Hour + 45*time.Minute, false},
		{"0s", 0, false},

		{"", 0, true},
		{"1x", 0, true},
		{"hour", 0, true},
		{"5 10d", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.span, func(t *testing.T) {
			actual, err := DurationFromTimeSpan(tt.span)

			if (err != nil) != tt.expectErr {
				t.Errorf("DurationFromTimeSpan(%q) error = %v, expectErr %v", tt.span, err, tt.expectErr)
				return
			}

			if !tt.expectErr && !durationsApproxEqual(actual, tt.expected, tolerance) {
				t.Errorf("DurationFromTimeSpan(%q) = %v, expected ~%v", tt.span, actual, tt.expected)
			}
		})
	}
}
