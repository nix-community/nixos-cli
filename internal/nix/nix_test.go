package nix_test

import (
	"slices"
	"testing"

	"github.com/nix-community/nixos-cli/internal/nix"
)

func TestMakeAttrName(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "a-z",
			input: "foobar",
			want:  "foobar",
		},
		{
			name:  "space and dot",
			input: "foo.bar ",
			want:  "\"foo.bar \"",
		},
		{
			name:  "space only",
			input: " ",
			want:  "\" \"",
		},
		{
			name:  "dot only",
			input: ".",
			want:  "\".\"",
		},
		{
			name:  "empty attribute",
			input: "",
			want:  "",
		},
		{
			name:  "already quoted",
			input: "\"foo.bar \"",
			want:  "\"foo.bar \"",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nix.MakeAttrName(tt.input)

			if got != tt.want {
				t.Errorf("Nix attribute name: got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestMakeAttrPath(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{
			name:  "no attributes",
			input: []string{},
			want:  "",
		},
		{
			name:  "single attribute",
			input: []string{"config"},
			want:  "config",
		},
		{
			name:  "multiple attributes",
			input: []string{"config", "foo", "bar"},
			want:  "config.foo.bar",
		},
		{
			name:  "attribute path",
			input: []string{"config", "config.foo.bar", "bar"},
			want:  "config.config.foo.bar.bar",
		},
		{
			name:  "space and dot",
			input: []string{"config", "foo. ", "bar"},
			want:  "config.foo.\" \".bar",
		},
		{
			name:  "already quoted",
			input: []string{"config", "\"foo. \"", "bar"},
			want:  "config.\"foo. \".bar",
		},
		{
			name:  "space only",
			input: []string{"config", " ", "bar"},
			want:  "config.\" \".bar",
		},
		{
			name:  "dot only",
			input: []string{"config", ".", "bar"},
			want:  "config.bar",
		},
		{
			name:  "escaped characters",
			input: []string{"config", "f\\\\o\\\"o\\.", "bar"},
			want:  "config.\"f\\o\"o.\".bar",
		},
		{
			name:  "empty attributes",
			input: []string{"", "config", "", "foo", "", "bar", ""},
			want:  "config.foo.bar",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nix.MakeAttrPath(tt.input...)

			if got != tt.want {
				t.Errorf("Nix attribute path: got %s, want %s", got, tt.want)
			}
		})
	}
}

func TestSplitAttrPath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "empty attribute",
			input: "",
			want:  []string{""},
		},
		{
			name:  "single attribute",
			input: "config",
			want:  []string{"config"},
		},
		{
			name:  "attribute path",
			input: "config.fo o.bar",
			want:  []string{"config", "fo o", "bar"},
		},
		{
			name:  "escaped characters",
			input: "config.f\\\\o\\\"o\\.bar",
			want:  []string{"config", "f\\o\"o.bar"},
		},
		{
			name:  "already quoted",
			input: "config.\"fo\\\\o.bar\"",
			want:  []string{"config", "fo\\o.bar"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nix.SplitAttrPath(tt.input)

			if !slices.Equal(got, tt.want) {
				t.Errorf("Nix attribute path: got %s, want %s", got, tt.want)
			}
		})
	}
}
