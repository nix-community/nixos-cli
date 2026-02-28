package ssh

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"testing"
)

func TestParseHostsFileLine(t *testing.T) {
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
			name: "single field",
			line: "127.0.0.1",
			want: map[string]struct{}{},
		},
		{
			name: "invalid ip",
			line: "0.0.0.0 blocked-site.com",
			want: map[string]struct{}{},
		},
		{
			name: "single host",
			line: "127.0.0.1 localhost",
			want: map[string]struct{}{"127.0.0.1": {}, "localhost": {}},
		},
		{
			name: "multiple hosts",
			line: "192.168.1.1 example.com test.local",
			want: map[string]struct{}{"192.168.1.1": {}, "example.com": {}, "test.local": {}},
		},
		{
			name: "extra spaces",
			line: "  192.168.1.1  example.com ",
			want: map[string]struct{}{"192.168.1.1": {}, "example.com": {}},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			hosts := make(map[string]struct{})
			parseHostsFileLine(test.line, hosts)

			got := slices.Sorted(maps.Keys(hosts))
			want := slices.Sorted(maps.Keys(test.want))
			if !slices.Equal(got, want) {
				t.Errorf("hosts: got %q, want %q", got, want)
			}
		})
	}
}

func TestParseConfigLine(t *testing.T) {
	tests := []struct {
		name               string
		line               string
		expectedHosts      map[string]struct{}
		expectedIncludes   []string
		expectedKnownHosts []string
	}{
		{
			name:               "empty",
			line:               "",
			expectedHosts:      nil,
			expectedIncludes:   []string{},
			expectedKnownHosts: []string{},
		},
		{
			name:               "comment",
			line:               "# comment",
			expectedHosts:      nil,
			expectedIncludes:   []string{},
			expectedKnownHosts: []string{},
		},
		{
			name:               "trailing comment",
			line:               "Host example.com # comment",
			expectedHosts:      map[string]struct{}{"example.com": {}},
			expectedIncludes:   []string{},
			expectedKnownHosts: []string{},
		},
		{
			name:               "host single",
			line:               "Host example.com",
			expectedHosts:      map[string]struct{}{"example.com": {}},
			expectedIncludes:   []string{},
			expectedKnownHosts: []string{},
		},
		{
			name:               "host multiple",
			line:               "Host host1 host2",
			expectedHosts:      map[string]struct{}{"host1": {}, "host2": {}},
			expectedIncludes:   []string{},
			expectedKnownHosts: []string{},
		},
		{
			name:               "host leading pattern",
			line:               "Host *.example.com",
			expectedHosts:      map[string]struct{}{},
			expectedIncludes:   []string{},
			expectedKnownHosts: []string{},
		},
		{
			name:               "host trailing pattern",
			line:               "Host example.com*",
			expectedHosts:      map[string]struct{}{"example.com": {}},
			expectedIncludes:   []string{},
			expectedKnownHosts: []string{},
		},
		{
			name:               "host equal",
			line:               "Host=example.com",
			expectedHosts:      map[string]struct{}{"example.com": {}},
			expectedIncludes:   []string{},
			expectedKnownHosts: []string{},
		},
		{
			name:               "host equals and space",
			line:               "Host   ==    example.com",
			expectedHosts:      map[string]struct{}{"example.com": {}},
			expectedIncludes:   []string{},
			expectedKnownHosts: []string{},
		},
		{
			name:               "host quoted",
			line:               "Host \"example.com\"",
			expectedHosts:      map[string]struct{}{"example.com": {}},
			expectedIncludes:   []string{},
			expectedKnownHosts: []string{},
		},
		{
			name:               "include",
			line:               "Include /path/to/config",
			expectedHosts:      nil,
			expectedIncludes:   []string{"/path/to/config"},
			expectedKnownHosts: []string{},
		},
		{
			name:               "global known hosts",
			line:               "GlobalKnownHostsFile /path/to/known_hosts",
			expectedHosts:      nil,
			expectedIncludes:   []string{},
			expectedKnownHosts: []string{"/path/to/known_hosts"},
		},
		{
			name:               "user known hosts",
			line:               "UserKnownHostsFile /path/to/known_hosts",
			expectedHosts:      nil,
			expectedIncludes:   []string{},
			expectedKnownHosts: []string{"/path/to/known_hosts"},
		},
		{
			name:               "include multiple quoted files",
			line:               "Include=\"some/config/file\"\t \"another/con fig=/file\"",
			expectedHosts:      nil,
			expectedIncludes:   []string{"some/config/file", "another/con fig=/file"},
			expectedKnownHosts: []string{},
		},
		{
			name:               "known hosts multiple quoted files",
			line:               "GlobalKnownHostsFile = \"some/known_hosts/file\"\t \"another/known_ hosts=/file\"",
			expectedHosts:      nil,
			expectedIncludes:   []string{},
			expectedKnownHosts: []string{"some/known_hosts/file", "another/known_ hosts=/file"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			hosts := make(map[string]struct{})
			includes, knownHosts := parseConfigLine(tt.line, hosts)

			hostsSlice := slices.Sorted(maps.Keys(hosts))
			expectedHostsSlice := slices.Sorted(maps.Keys(tt.expectedHosts))
			if !slices.Equal(hostsSlice, expectedHostsSlice) {
				t.Errorf("hosts: got %q, want %q", hostsSlice, expectedHostsSlice)
			}

			if !slices.Equal(includes, tt.expectedIncludes) {
				t.Errorf("includes: got %q, want %q", includes, tt.expectedIncludes)
			}

			if !slices.Equal(knownHosts, tt.expectedKnownHosts) {
				t.Errorf("knownHosts: got %q, want %q", knownHosts, tt.expectedKnownHosts)
			}
		})
	}
}

func TestResolveIncludePath(t *testing.T) {
	home, _ := os.UserHomeDir()

	tests := []struct {
		name   string
		path   string
		system bool
		want   []string
	}{
		{
			name:   "tilde user",
			path:   "~/foo",
			system: false,
			want:   []string{filepath.Join(home, "foo")},
		},
		{
			name:   "tilde system",
			path:   "~/foo",
			system: true,
			want:   []string{filepath.Join(SystemConfigPrefix, "~/foo")},
		},
		{
			name:   "relative user",
			path:   "config.d/foo",
			system: false,
			want:   []string{filepath.Join(home, ".ssh", "config.d/foo")},
		},
		{
			name:   "relative system",
			path:   "ssh_config.d/foo",
			system: true,
			want:   []string{filepath.Join(SystemConfigPrefix, "ssh_config.d/foo")},
		},
		{
			name:   "absolute user",
			path:   "/home/.ssh_extra/foo",
			system: false,
			want:   []string{"/home/.ssh_extra/foo"},
		},
		{
			name:   "absolute system",
			path:   "/etc/ssh_extra/foo",
			system: true,
			want:   []string{"/etc/ssh_extra/foo"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolveIncludePath(tt.path, home, tt.system)

			if !slices.Equal(got, tt.want) {
				t.Errorf("resolveIncludePath: got %q, want %q", got, tt.want)
			}
		})
	}
}

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
