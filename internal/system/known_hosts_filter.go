package system

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"strings"

	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"
)

// A known_hosts entry with a key type `x/crypto/ssh` cannot parse.
//
// `knownhosts.New` rejects the entire file on encountering one,
// so these are filtered out beforehand.
type unsupportedKnownHost struct {
	file    string
	line    int
	hosts   []string
	keyType string
}

func isUnsupportedKeyErr(err error) bool {
	return err != nil && strings.Contains(err.Error(), "unknown key algorithm")
}

// Extract the host patterns and key type from a known_hosts line,
// skipping the optional `@marker`.
func splitKnownHostsLine(line string) (hosts []string, keyType string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) > 0 && strings.HasPrefix(fields[0], "@") {
		fields = fields[1:]
	}
	if len(fields) < 3 {
		return nil, "", false
	}
	return strings.Split(fields[0], ","), fields[1], true
}

// Return paths that are safe to pass to `knownhosts.New`, along with
// the entries that had to be dropped.
//
// Files with no unsupported entries are passed through as-is. Otherwise,
// a filtered temporary copy is written, which the returned cleanup
// function removes.
//
// Lines that are malformed for any other reason are kept, so that they
// still fail loudly.
func sanitizeKnownHosts(log logger.Logger, files []string) (paths []string, skipped []unsupportedKnownHost, cleanup func(), err error) {
	var temps []string
	cleanup = func() {
		for _, t := range temps {
			_ = os.Remove(t)
		}
	}

	for _, f := range files {
		data, readErr := os.ReadFile(f)
		if readErr != nil {
			// let knownhosts.New report unreadable files as before
			paths = append(paths, f)
			continue
		}

		var kept []string
		var fileSkipped []unsupportedKnownHost
		for i, line := range strings.Split(string(data), "\n") {
			trimmed := strings.TrimSpace(line)
			if trimmed == "" || strings.HasPrefix(trimmed, "#") {
				kept = append(kept, line)
				continue
			}
			_, _, _, _, _, perr := ssh.ParseKnownHosts([]byte(trimmed))
			if !isUnsupportedKeyErr(perr) {
				kept = append(kept, line)
				continue
			}
			if hosts, kt, ok := splitKnownHostsLine(trimmed); ok {
				fileSkipped = append(fileSkipped, unsupportedKnownHost{file: f, line: i + 1, hosts: hosts, keyType: kt})
				log.Debugf("skipping %s:%d: unsupported host key type %s", f, i+1, kt)
			} else {
				kept = append(kept, line)
			}
		}

		if len(fileSkipped) == 0 {
			paths = append(paths, f)
			continue
		}

		log.Warnf("skipped %d known_hosts entries in %s with unsupported key types", len(fileSkipped), f)
		skipped = append(skipped, fileSkipped...)

		tmp, terr := os.CreateTemp("", "nixos-known-hosts-*")
		if terr != nil {
			cleanup()
			return nil, nil, func() {}, fmt.Errorf("failed to create temporary known hosts file: %w", terr)
		}
		temps = append(temps, tmp.Name())
		_, werr := tmp.WriteString(strings.Join(kept, "\n"))
		cerr := tmp.Close()
		if werr != nil || cerr != nil {
			cleanup()
			return nil, nil, func() {}, fmt.Errorf("failed to write temporary known hosts file: %w", errors.Join(werr, cerr))
		}
		paths = append(paths, tmp.Name())
	}

	return paths, skipped, cleanup, nil
}

func hostPatternMatches(pattern string, names []string) bool {
	if strings.HasPrefix(pattern, "|1|") {
		parts := strings.Split(pattern, "|")
		if len(parts) != 4 {
			return false
		}
		salt, err1 := base64.StdEncoding.DecodeString(parts[2])
		want, err2 := base64.StdEncoding.DecodeString(parts[3])
		if err1 != nil || err2 != nil {
			return false
		}
		for _, n := range names {
			mac := hmac.New(sha1.New, salt)
			mac.Write([]byte(n))
			if hmac.Equal(mac.Sum(nil), want) {
				return true
			}
		}
		return false
	}
	if strings.HasPrefix(pattern, "!") {
		return false
	}
	for _, n := range names {
		if ok, _ := path.Match(pattern, n); ok {
			return true
		}
	}
	return false
}

func (u unsupportedKnownHost) matches(names []string) bool {
	matched := false
	for _, p := range u.hosts {
		if strings.HasPrefix(p, "!") && hostPatternMatches(p[1:], names) {
			return false
		}
		if hostPatternMatches(p, names) {
			matched = true
		}
	}
	return matched
}

// Wrap HostKeyCallback to error for hosts whose only known keys were skipped.
func failOnUnsupportedOnly(
	next ssh.HostKeyCallback,
	skipped []unsupportedKnownHost,
	hostKeyVerification settings.HostKeyVerificationType,
) ssh.HostKeyCallback {
	if len(skipped) == 0 {
		return next
	}

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		err := next(hostname, remote, key)
		if err == nil || hostKeyVerification == settings.HostKeyVerificationOff {
			return err
		}

		keyErr, ok := errors.AsType[*knownhosts.KeyError](err)
		if !ok || len(keyErr.Want) > 0 {
			return err
		}

		names := []string{knownhosts.Normalize(hostname)}
		if remote != nil {
			names = append(names, knownhosts.Normalize(remote.String()))
		}

		var found []string
		for _, s := range skipped {
			if s.matches(names) {
				found = append(found, fmt.Sprintf("%s:%d (%s)", s.file, s.line, s.keyType))
			}
		}
		if len(found) == 0 {
			return err
		}

		return fmt.Errorf(
			"the only known host keys for %s use key types this build of nixos-cli cannot verify:\n  %s\n"+
				"remove them, or record a supported key (e.g. ssh-keyscan -t ed25519 <host> >> ~/.ssh/known_hosts)",
			hostname, strings.Join(found, "\n  "),
		)
	}
}
