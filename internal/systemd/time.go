package systemdUtils

import (
	"fmt"
	"slices"
	"strconv"
	"time"
	"unicode"
)

type SystemdDuration time.Duration

// Parse a duration from a systemd.time(7) string.
//
// Returns a SystemdDuration, a defined type wrapping time.Duration.
func DurationFromTimeSpan(span string) (SystemdDuration, error) {
	if len(span) < 2 {
		return 0, fmt.Errorf("time span too short")
	}

	for _, c := range span {
		if !unicode.IsDigit(c) && !unicode.IsLetter(c) && c != ' ' {
			return 0, fmt.Errorf("invalid character %v", c)
		}
	}

	if !unicode.IsDigit(rune(span[0])) {
		return 0, fmt.Errorf("span must start with number")
	}

	totalDuration := time.Duration(0)

	i := 0
	spanLen := len(span)

	for i < spanLen {
		if span[i] == ' ' {
			i += 1
			continue
		}
		if !unicode.IsDigit(rune(span[i])) {
			return 0, fmt.Errorf("span components must start with numbers")
		}

		numStart := i
		for i < spanLen && unicode.IsDigit(rune(span[i])) {
			i += 1
		}
		num, _ := strconv.ParseInt(span[numStart:i], 10, 64)

		if i >= spanLen {
			return 0, fmt.Errorf("span components must have units")
		}

		for unicode.IsSpace(rune(span[i])) {
			i += 1
		}

		unitStart := i
		for i < spanLen && unicode.IsLetter(rune(span[i])) {
			i += 1
		}
		unit := span[unitStart:i]

		var durationUnit time.Duration
		if slices.Contains([]string{"ns", "nsec"}, unit) {
			durationUnit = time.Nanosecond
		} else if slices.Contains([]string{"us", "usec"}, unit) {
			durationUnit = time.Microsecond
		} else if slices.Contains([]string{"ms", "msec"}, unit) {
			durationUnit = time.Millisecond
		} else if slices.Contains([]string{"s", "sec", "second", "seconds"}, unit) {
			durationUnit = time.Second
		} else if slices.Contains([]string{"m", "min", "minute", "minutes"}, unit) {
			durationUnit = time.Minute
		} else if slices.Contains([]string{"h", "hr", "hour", "hours"}, unit) {
			durationUnit = time.Hour
		} else if slices.Contains([]string{"d", "day", "days"}, unit) {
			durationUnit = time.Hour * 24
		} else if slices.Contains([]string{"w", "week", "weeks"}, unit) {
			durationUnit = time.Hour * 24 * 7
		} else if slices.Contains([]string{"M", "month", "months"}, unit) {
			durationUnit = time.Duration(30.44 * float64(24) * float64(time.Hour))
		} else if slices.Contains([]string{"y", "year", "years"}, unit) {
			durationUnit = time.Duration(365.25 * float64(24) * float64(time.Hour))
		} else {
			return 0, fmt.Errorf("invalid unit")
		}

		totalDuration += time.Duration(num) * durationUnit
	}

	return SystemdDuration(totalDuration), nil
}

func (s SystemdDuration) Duration() time.Duration {
	return time.Duration(s)
}

// cobra value parsing implementation
func (d *SystemdDuration) Set(value string) error {
	parsed, err := DurationFromTimeSpan(value)
	if err != nil {
		return err
	}

	*d = parsed
	return nil
}

func (d *SystemdDuration) String() string {
	return time.Duration(*d).String()
}

func (d *SystemdDuration) Type() string {
	return "systemd-duration"
}

// koanf unmarshaling support
func (d *SystemdDuration) UnmarshalText(text []byte) error {
	parsed, err := DurationFromTimeSpan(string(text))
	if err != nil {
		return err
	}

	*d = parsed
	return nil
}
