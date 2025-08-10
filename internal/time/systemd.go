package time

import (
	"fmt"
	"strconv"
	"time"
	"unicode"
)

// Parse a time.Duration from a systemd.time(7) string.
func DurationFromTimeSpan(span string) (time.Duration, error) {
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
		if containsSlice(unit, []string{"ns", "nsec"}) {
			durationUnit = time.Nanosecond
		} else if containsSlice(unit, []string{"us", "usec"}) {
			durationUnit = time.Microsecond
		} else if containsSlice(unit, []string{"ms", "msec"}) {
			durationUnit = time.Millisecond
		} else if containsSlice(unit, []string{"s", "sec", "second", "seconds"}) {
			durationUnit = time.Second
		} else if containsSlice(unit, []string{"m", "min", "minute", "minutes"}) {
			durationUnit = time.Minute
		} else if containsSlice(unit, []string{"h", "hr", "hour", "hours"}) {
			durationUnit = time.Hour
		} else if containsSlice(unit, []string{"d", "day", "days"}) {
			durationUnit = time.Hour * 24
		} else if containsSlice(unit, []string{"w", "week", "weeks"}) {
			durationUnit = time.Hour * 24 * 7
		} else if containsSlice(unit, []string{"M", "month", "months"}) {
			durationUnit = time.Duration(30.44 * float64(24) * float64(time.Hour))
		} else if containsSlice(unit, []string{"y", "year", "years"}) {
			durationUnit = time.Duration(365.25 * float64(24) * float64(time.Hour))
		} else {
			return 0, fmt.Errorf("invalid unit")
		}

		totalDuration += time.Duration(num) * durationUnit
	}

	return totalDuration, nil
}

func containsSlice(candidate string, candidates []string) bool {
	for _, v := range candidates {
		if v == candidate {
			return true
		}
	}
	return false
}
