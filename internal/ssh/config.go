package ssh

import (
	"bufio"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/kevinburke/hostsfile/lib"
	"github.com/kevinburke/ssh_config"
)

const (
	hostsFileMaxSize = 1 << 20

	configPattern     = "*?!"
	knownHostsPattern = "*?!"

	SystemConfigFile     = "/etc/ssh/ssh_config"
	SystemKnownHostsFile = "/etc/ssh/ssh_known_hosts"
	UserConfigFile       = ".ssh/config"
	UserKnownHostsFile   = ".ssh/known_hosts"
)

// Get a sorted, deduplicated list of hosts from:
// - The hosts file (if `useHostsFile` is set)
// - SSH config files
// - SSH known_hosts files
func getHosts(useHostsFile bool) ([]string, error) {
	hosts := make(map[string]struct{})

	if useHostsFile {
		if err := getHostsFileHosts(hosts); err != nil && !os.IsNotExist(err) {
			return []string{}, err
		}
	}

	configFiles := []string{SystemConfigFile}
	// The default known hosts files can be specified in the config files,
	// so a set is used to avoid reading them multiple times.
	knownHostsFiles := map[string]struct{}{
		SystemKnownHostsFile: struct{}{},
	}

	home, err := os.UserHomeDir()
	if err == nil {
		configFiles = append(configFiles, filepath.Join(home, UserConfigFile))
		knownHostsFiles[filepath.Join(home, UserKnownHostsFile)] = struct{}{}
	}

	for _, f := range configFiles {
		if err = getConfigHosts(f, hosts, knownHostsFiles); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return []string{}, err
		}
	}

	for _, f := range slices.Sorted(maps.Keys(knownHostsFiles)) {
		if err = getKnownHosts(expandTilde(f, home), hosts); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return []string{}, err
		}
	}

	return slices.Sorted(maps.Keys(hosts)), nil
}

func getHostsFileHosts(hosts map[string]struct{}) error {
	file, err := os.Open(hostsfile.Location)
	if err != nil {
		return err
	}

	// Don't process overly large hosts files as they are probably blocklists.
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	} else if fileInfo.Size() > hostsFileMaxSize {
		return nil
	}

	h, err := hostsfile.Decode(file)
	if err != nil {
		return err
	}
	_ = file.Close()

	for _, record := range h.Records() {
		ip := record.IpAddress.String()
		// Empty IP is returned for a comment/blank line
		// Invalid IP usually indicates a blocked host
		if ip == "" || ip == "0.0.0.0" {
			continue
		}
		hosts[ip] = struct{}{}
		for host := range maps.Keys(record.Hostnames) {
			hosts[host] = struct{}{}
		}
	}

	return nil
}

// Add the hosts from an SSH config file to the `hosts` set
// and any known hosts files to `knownHosts`.
func getConfigHosts(path string, hosts map[string]struct{}, knownHosts map[string]struct{}) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}

	cfg, _ := ssh_config.Decode(file)
	_ = file.Close()

	addHosts := func(host *ssh_config.Host) {
		for _, pattern := range host.Patterns {
			// Allow trailing patterns
			val := strings.TrimRight(pattern.String(), configPattern)
			// Skip empty hosts and hosts with leading patterns
			if val == "" || strings.IndexAny(val, configPattern) == 0 {
				continue
			}
			hosts[val] = struct{}{}
		}
	}

	for _, host := range cfg.Hosts {
		addHosts(host)
		for _, node := range host.Nodes {
			switch t := node.(type) {
			case *ssh_config.Include:
				for _, cfg := range t.Files {
					for _, host := range cfg.Hosts {
						addHosts(host)
					}
				}
			case *ssh_config.KV:
				key := t.Key
				if key == "globalknownhostsfile" || key == "userknownhostsfile" {
					knownHosts[t.Value] = struct{}{}
				}
			}
		}
	}

	return nil
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
		if strings.ContainsAny(v, knownHostsPattern) {
			return
		}
		// Extract host enclosed in '[]:port'
		if i := strings.LastIndex(v, "]:"); i > 1 && v[0] == '[' {
			v = v[1:i]
		}
		hosts[v] = struct{}{}
	}
}
