package ssh

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type UserHostPort struct {
	User string
	Host string
	Port int
}

func ParseUserHostPort(s string) (*UserHostPort, error) {
	var user string
	if at := strings.LastIndex(s, "@"); at != -1 {
		user = s[:at]
		s = s[at+1:]
	}

	// First, check if [] is a closed pair.
	// If it is, then we can enforce an IPV6 address
	// and parse it on our own.
	// If any brackets are unmatched or if there
	// are multiple, then error out.
	// The port must also be valid here.
	// Bracketed IPv6 handling
	if strings.ContainsAny(s, "[]") {
		// Must be exactly one opening and one closing bracket
		if strings.Count(s, "[") != 1 || strings.Count(s, "]") != 1 {
			return nil, fmt.Errorf("invalid IPv6 address format; mismatched or multiple brackets detected")
		}

		// Must start with '['
		if !strings.HasPrefix(s, "[") {
			return nil, fmt.Errorf("invalid IPv6 address format; missing [")
		}

		end := strings.Index(s, "]")
		if end == -1 {
			return nil, fmt.Errorf("invalid IPv6 address format; missing ]")
		}

		hostPart := s[1:end]
		rest := s[end+1:]

		// Optional port
		var port int
		if rest != "" {
			if !strings.HasPrefix(rest, ":") {
				return nil, fmt.Errorf("invalid host format")
			}

			p, err := parsePort(rest[1:])
			if err != nil {
				return nil, err
			}
			port = p
		}

		return &UserHostPort{
			User: user,
			Host: hostPart,
			Port: port,
		}, nil
	}

	// Everything else that does not follow this rule,
	// so attempt to parse it as a hostname:port pair.
	var host, portStr string
	host, portStr, err := net.SplitHostPort(s)
	if err != nil {
		host = s
	}

	var port int
	if portStr != "" {
		var parsedPort int

		parsedPort, err = parsePort(portStr)
		if err != nil {
			return nil, err
		}

		port = parsedPort
	}

	ret := &UserHostPort{
		User: user,
		Host: host,
		Port: port,
	}
	return ret, nil
}

func parsePort(input string) (int, error) {
	p, err := strconv.ParseUint(input, 10, 16)
	if err != nil {
		return 0, errors.New("port must be between 1-65535")
	}

	if p == 0 || p > 65535 {
		return 0, errors.New("port must be between 1-65535")
	}

	return int(p), nil
}
