package system

import (
	"crypto/ed25519"
	"crypto/rand"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

func testKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, _ := ed25519.GenerateKey(rand.Reader)
	k, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatal(err)
	}
	return k
}

func build(t *testing.T, content string) ssh.HostKeyCallback {
	t.Helper()
	f := filepath.Join(t.TempDir(), "known_hosts")
	if err := os.WriteFile(f, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	paths, skipped, cleanup, err := sanitizeKnownHosts(logger.NewConsoleLogger(), []string{f})
	if err != nil {
		t.Fatal(err)
	}
	defer cleanup()
	cb, err := knownhosts.New(paths...)
	if err != nil {
		t.Fatalf("knownhosts.New: %v", err)
	}
	return failOnUnsupportedOnly(cb, skipped, settings.HostKeyVerificationAsk)
}

const unsupportedKey = "ssh-nixos-cli-test-unsupported AAAAHnNzaC1uaXhvcy1jbGktdGVzdC11bnN1cHBvcnRlZAAAAAAAAAAA"

func TestSkipsUnsupportedAndKeepsSupported(t *testing.T) {
	good := testKey(t)
	line := knownhosts.Line([]string{"good.example"}, good)
	cb := build(t, "10.0.0.1 "+unsupportedKey+"\n"+line+"\n")
	addr := &net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 22}
	if err := cb("good.example:22", addr, good); err != nil {
		t.Fatalf("supported entry should verify: %v", err)
	}
}

func TestOnlyUnsupportedFailsLoud(t *testing.T) {
	cb := build(t, "oracle.example "+unsupportedKey+"\n")
	addr := &net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 22}
	err := cb("oracle.example:22", addr, testKey(t))
	if err == nil || !strings.Contains(err.Error(), "cannot verify") {
		t.Fatalf("want unsupported-only error, got %v", err)
	}
}

func TestUnrelatedHostStillUnknown(t *testing.T) {
	cb := build(t, "oracle.example "+unsupportedKey+"\n")
	addr := &net.TCPAddr{IP: net.ParseIP("1.2.3.4"), Port: 22}
	err := cb("other.example:22", addr, testKey(t))
	if _, ok := err.(*knownhosts.KeyError); !ok {
		t.Fatalf("want plain KeyError so TOFU still applies, got %v", err)
	}
}
