package ssh

import (
	"maps"
	"slices"
	"testing"
)

func TestParseKnownHostsLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want map[string]struct{}
	}{
		{
			name: "empty",
			line: "",
			want: map[string]struct{}{},
		},
		{
			name: "comment",
			line: "# comment",
			want: map[string]struct{}{},
		},
		{
			name: "hashed host",
			line: "|1|random|hashedhostname ssh-rsa AAAA1234.....=",
			want: map[string]struct{}{},
		},
		{
			name: "insufficient fields",
			line: "example.com",
			want: map[string]struct{}{},
		},
		{
			name: "single host",
			line: "example.com ssh-rsa AAAA1234.....=",
			want: map[string]struct{}{"example.com": {}},
		},
		{
			name: "multiple hosts",
			line: "host1,host2 ssh-rsa AAAA1234.....=",
			want: map[string]struct{}{"host1": {}, "host2": {}},
		},
		{
			name: "marker",
			line: "@cert-authority example.com ssh-rsa AAAA1234.....=",
			want: map[string]struct{}{"example.com": {}},
		},
		{
			name: "host with port",
			line: "[192.168.1.1]:22 ssh-rsa AAAA1234.....=",
			want: map[string]struct{}{"192.168.1.1": {}},
		},
		{
			name: "host with pattern",
			line: "*.example.com ssh-rsa AAAA1234.....=",
			want: map[string]struct{}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hosts := make(map[string]struct{})
			parseKnownHostsLine(tt.line, hosts)

			got := slices.Sorted(maps.Keys(hosts))
			want := slices.Sorted(maps.Keys(tt.want))

			if !slices.Equal(got, want) {
				t.Errorf("hosts: got %q, want %q", got, want)
			}
		})
	}
}
