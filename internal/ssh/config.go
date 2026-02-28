package ssh

import (
	"bufio"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
)

const (
	HostsFileMaxSize = 1 << 20
	HostsFile        = "/etc/hosts"

	// Max recursion depth as per openssh's READCONF_MAX_DEPTH
	// https://anongit.mindrot.org/openssh.git/tree/readconf.c?id=487cf4c18c123b66c1f3f733398cd37e6b2ab6ab#n2558
	ConfigMaxDepth    = 16
	ConfigPattern     = "*?!"
	KnownHostsPattern = "*?!"

	SystemConfigPrefix   = "/etc/ssh"
	SystemConfigFile     = "/etc/ssh/ssh_config"
	SystemKnownHostsFile = "/etc/ssh/ssh_known_hosts"
	UserConfigFolder     = ".ssh"
	UserConfigFile       = ".ssh/config"
	UserKnownHostsFile   = ".ssh/known_hosts"
)

// Get a sorted, deduplicated list of hosts from:
// - The hosts file
// - SSH config files
// - SSH known_hosts files
func getHosts(useHostsFile bool) ([]string, error) {
	hosts := make(map[string]struct{})

	if useHostsFile {
		if err := getHostsFileHosts(HostsFile, hosts); err != nil {
			return []string{}, err
		}
	}

	// A map of configFile -> isSystemConfig
	// Necessary for resolving Include paths
	sshConfigFiles := map[string]bool{
		SystemConfigFile: true,
	}
	sshKnownHostsFiles := []string{SystemKnownHostsFile}

	home, err := os.UserHomeDir()
	if err == nil {
		sshConfigFiles[filepath.Join(home, UserConfigFile)] = false
		sshKnownHostsFiles = append(sshKnownHostsFiles, filepath.Join(home, UserKnownHostsFile))
	}

	for _, f := range slices.Sorted(maps.Keys(sshConfigFiles)) {
		if err = getConfigHosts(f, hosts, home, sshConfigFiles[f], 0); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return []string{}, err
		}
	}

	for _, f := range sshKnownHostsFiles {
		if err = getKnownHosts(f, hosts); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return []string{}, err
		}
	}

	return slices.Sorted(maps.Keys(hosts)), nil
}

func getHostsFileHosts(path string, hosts map[string]struct{}) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	// Limit the hosts file size in case its being used as a blocklist
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	} else if fileInfo.Size() > HostsFileMaxSize {
		return nil
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parseHostsFileLine(scanner.Text(), hosts)
	}

	return scanner.Err()
}

func parseHostsFileLine(line string, hosts map[string]struct{}) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return
	}

	if i := strings.Index(line, "#"); i != -1 {
		line = strings.TrimSpace(line[:i])
	}

	fields := strings.Fields(line)
	// Skip single field or invalid ip
	if len(fields) < 2 || fields[0] == "0.0.0.0" {
		return
	}

	hosts[fields[0]] = struct{}{}

	for _, host := range fields[1:] {
		hosts[host] = struct{}{}
	}
}

func getConfigHosts(path string, hosts map[string]struct{}, home string, system bool, depth int) error {
	if depth > ConfigMaxDepth {
		return nil
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		var includes []string
		var knownHosts []string
		includes, knownHosts = parseConfigLine(scanner.Text(), hosts)

		for _, include := range includes {
			resolved := resolveIncludePath(include, home, system)

			for _, match := range resolved {
				if err = getConfigHosts(match, hosts, home, system, depth+1); err != nil {
					if os.IsNotExist(err) {
						continue
					}
					return err
				}
			}
		}

		for _, knownHost := range knownHosts {
			if err = getKnownHosts(expandTilde(knownHost, home), hosts); err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
		}
	}

	return scanner.Err()
}

var knownConfigKeys = []string{"host", "include", "globalknownhostsfile", "userknownhostsfile"}

func parseConfigLine(line string, hosts map[string]struct{}) (includes, knownHosts []string) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return nil, nil
	}

	if i := strings.Index(line, "#"); i != -1 {
		line = strings.TrimSpace(line[:i])
	}

	key, value, found := splitConfigLine(knownConfigKeys, line)
	if !found {
		return nil, nil
	}

	switch key {
	case "host":
		for _, v := range splitConfigValue(value) {
			// Allow trailing patterns
			v = strings.TrimRight(v, ConfigPattern)
			// Skip empty hosts and hosts with leading patterns
			if v == "" || strings.IndexAny(v, ConfigPattern) == 0 {
				continue
			}
			hosts[v] = struct{}{}
		}
	case "include":
		includes = append(includes, splitConfigValue(value)...)
	case "globalknownhostsfile", "userknownhostsfile":
		knownHosts = append(knownHosts, splitConfigValue(value)...)
	}

	return includes, knownHosts
}

// Extract the key and value from an SSH config line, where they can
// be separated by whitespace or '=' with optional surrounding whitespace.
// Uses the provided list of recognized keys to match the key.
func splitConfigLine(keys []string, line string) (key, value string, found bool) {
	var currentKey strings.Builder

	line = strings.TrimSpace(line)

	for i, r := range line {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			currentKey.WriteRune(unicode.ToLower(r))
		} else if r == '=' || unicode.IsSpace(r) {
			if currentKey.Len() > 0 {
				key = currentKey.String()
				if !slices.Contains(keys, key) {
					return key, "", false
				}
				found = true

				// Skip whitespace and '='
				for i < len(line) && (line[i] == ' ' || line[i] == '\t' || line[i] == '=') {
					i++
				}
				value = line[i:]

				break
			}
		}
	}

	return key, value, found
}

// Extract the space separated and optionally quoted values from an
// SSH config value.
func splitConfigValue(value string) []string {
	var parts []string
	var str strings.Builder

	value = strings.TrimSpace(value)
	quoted := false

	for _, r := range value {
		switch r {
		case '"':
			quoted = !quoted

		case ' ', '\t':
			if quoted {
				str.WriteRune(r)
			} else if str.Len() > 0 {
				parts = append(parts, str.String())
				str.Reset()
			}

		default:
			str.WriteRune(r)
		}
	}

	if str.Len() > 0 {
		parts = append(parts, str.String())
	}

	return parts
}

// Resolve Include paths according to ssh_config(5):
// "each pathname may contain glob(7) wildcards, ... and, for user
// configurations, shell-like ‘~’ references to user home directories
// ... Files without absolute paths are assumed to be in ~/.ssh
// if included in a user configuration file or /etc/ssh if included
// from the system configuration file"
// Do not handle expansion of environment variables or tokens.
func resolveIncludePath(path string, home string, system bool) []string {
	if !system {
		path = expandTilde(path, home)
	}

	if !filepath.IsAbs(path) {
		if system {
			path = filepath.Join(SystemConfigPrefix, path)
		} else {
			path = filepath.Join(home, UserConfigFolder, path)
		}
	}

	matches, err := filepath.Glob(path)
	if len(matches) == 0 || err != nil {
		return []string{path}
	}
	return matches
}

func expandTilde(path string, home string) string {
	// Do not expand path = "~" as that cannot be a config or known hosts file
	if strings.HasPrefix(path, "~/") {
		return filepath.Join(home, strings.TrimPrefix(path, "~/"))
	}
	return path
}

func getKnownHosts(path string, hosts map[string]struct{}) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		parseKnownHostsLine(scanner.Text(), hosts)
	}

	return scanner.Err()
}

// https://man.openbsd.org/sshd.8#SSH_KNOWN_HOSTS_FILE_FORMAT
func parseKnownHostsLine(line string, hosts map[string]struct{}) {
	line = strings.TrimSpace(line)

	// Skip empty + commented lines, and hashed hosts
	if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "|") {
		return
	}

	// Must contain at least hosts, keytype and key
	fields := strings.Fields(line)
	if len(fields) < 3 {
		return
	}

	// Ignore leading markers
	if strings.HasPrefix(fields[0], "@") {
		fields = fields[1:]
	}

	for _, v := range strings.Split(fields[0], ",") {
		if strings.ContainsAny(v, KnownHostsPattern) {
			return
		}
		// Extract host enclosed in '[]:port'
		if i := strings.LastIndex(v, "]:"); i > 1 && v[0] == '[' {
			v = v[1:i]
		}
		hosts[v] = struct{}{}
	}
}
