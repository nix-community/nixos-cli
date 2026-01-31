package nix

import (
	"regexp"
	"strings"
)

var specialAttrCharsPattern = regexp.MustCompile(`[. ]`)

// Create a Nix attribute name by quoting the string s if it contains
// dots or spaces.
func MakeAttrName(s string) string {
	if strings.HasPrefix(s, "\"") && strings.HasSuffix(s, "\"") ||
		!specialAttrCharsPattern.MatchString(s) {
		return s
	}

	return "\"" + s + "\""
}

// Create a Nix attribute path by joining attribute names with dots
// and ignoring any empty attributes names.
func MakeAttrPath[T ~string](values ...T) string {
	var attrPath strings.Builder

	for i := range values {
		for _, value := range SplitAttrPath(string(values[i])) {
			next := MakeAttrName(value)

			if next != "" {
				if attrPath.Len() > 0 {
					attrPath.WriteString(".")
				}
				attrPath.WriteString(next)
			}
		}
	}

	return attrPath.String()
}

// Split an attribute path into its constituent components,
// using '.' as a separator (if it is not enclosed in quotes),
// and '\' as an escape character.
func SplitAttrPath(s string) []string {
	var parts []string
	var str strings.Builder

	escaped := false
	quoted := false

	for _, r := range s {
		if escaped {
			str.WriteRune(r)
			escaped = false
			continue
		}

		switch {
		case r == '\\':
			escaped = true

		case r == '"':
			quoted = !quoted

		case r == '.' && !quoted:
			parts = append(parts, str.String())
			str.Reset()

		default:
			str.WriteRune(r)
		}
	}

	parts = append(parts, str.String())
	return parts
}
