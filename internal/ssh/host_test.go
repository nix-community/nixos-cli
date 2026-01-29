package ssh_test

import (
	"testing"

	sshUtils "github.com/nix-community/nixos-cli/internal/ssh"
)

func TestParseUserHostPort(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *sshUtils.UserHostPort
		wantErr bool
	}{
		{
			name:  "host only",
			input: "example.com",
			want: &sshUtils.UserHostPort{
				User: "",
				Host: "example.com",
				Port: 0,
			},
		},
		{
			name:  "host and port",
			input: "example.com:22",
			want: &sshUtils.UserHostPort{
				User: "",
				Host: "example.com",
				Port: 22,
			},
		},
		{
			name:  "user host and port",
			input: "alice@example.com:2222",
			want: &sshUtils.UserHostPort{
				User: "alice",
				Host: "example.com",
				Port: 2222,
			},
		},
		{
			name:  "ipv4 and port",
			input: "192.168.1.10:22",
			want: &sshUtils.UserHostPort{
				User: "",
				Host: "192.168.1.10",
				Port: 22,
			},
		},
		{
			name:  "bracketed ipv6 without port",
			input: "[2001:db8::1]",
			want: &sshUtils.UserHostPort{
				User: "",
				Host: "2001:db8::1",
				Port: 0,
			},
		},
		{
			name:  "bracketed ipv6 with port",
			input: "[2001:db8::1]:22",
			want: &sshUtils.UserHostPort{
				User: "",
				Host: "2001:db8::1",
				Port: 22,
			},
		},
		{
			name:  "user and bracketed ipv6 with port",
			input: "bob@[2001:db8::1]:2222",
			want: &sshUtils.UserHostPort{
				User: "bob",
				Host: "2001:db8::1",
				Port: 2222,
			},
		},
		{
			name:    "invalid port",
			input:   "example.com:99999",
			wantErr: true,
		},
		{
			name:    "non numeric port",
			input:   "example.com:http",
			wantErr: true,
		},
		{
			name:  "unbracketed ipv6 without port",
			input: "2001:db8::1",
			want: &sshUtils.UserHostPort{
				User: "",
				Host: "2001:db8::1",
				Port: 0,
			},
		},
		{
			name:    "invalid bracketed ipv6 missing closing bracket",
			input:   "[2001:db8::1",
			wantErr: true,
		},
		{
			name:    "invalid bracketed ipv6 junk after bracket",
			input:   "[2001:db8::1]junk",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := sshUtils.ParseUserHostPort(tt.input)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if got.User != tt.want.User {
				t.Errorf("User: got %q, want %q", got.User, tt.want.User)
			}
			if got.Host != tt.want.Host {
				t.Errorf("Host: got %q, want %q", got.Host, tt.want.Host)
			}
			if got.Port != tt.want.Port {
				t.Errorf("Port: got %d, want %d", got.Port, tt.want.Port)
			}
		})
	}
}
