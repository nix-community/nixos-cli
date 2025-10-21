package system

import (
	"bufio"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"github.com/nix-community/nixos-cli/internal/logger"
)

type LocalSystem struct {
	logger logger.Logger
}

func NewLocalSystem(logger logger.Logger) *LocalSystem {
	return &LocalSystem{
		logger: logger,
	}
}

func (l *LocalSystem) Run(cmd *Command) (int, error) {
	command := exec.Command(cmd.Name, cmd.Args...)

	command.Stdout = cmd.Stdout
	command.Stderr = cmd.Stderr
	command.Stdin = cmd.Stdin
	command.Env = os.Environ()

	for key, value := range cmd.Env {
		command.Env = append(command.Env, key+"="+value)
	}

	err := command.Run()

	if exitErr, ok := err.(*exec.ExitError); ok {
		if status, ok := exitErr.Sys().(interface{ ExitStatus() int }); ok {
			return status.ExitStatus(), err
		}
	}

	if err == nil {
		return 0, nil
	}

	return 0, err
}

var nixosDistroIDRegex = regexp.MustCompile("^\"?nixos\"?$")

func (l *LocalSystem) IsNixOS() bool {
	_, err := os.Stat("/etc/NIXOS")
	if err == nil {
		return true
	}

	osRelease, err := parseOSRelease()
	if err != nil {
		return false
	}

	distroID, ok := osRelease["ID"]
	if !ok {
		return false
	}

	return nixosDistroIDRegex.MatchString(distroID)
}

func (l *LocalSystem) Logger() logger.Logger {
	return l.logger
}

func parseOSRelease() (map[string]string, error) {
	values := make(map[string]string)

	osRelease, err := os.Open("/etc/os-release")
	if err != nil {
		return nil, err
	}
	defer func() { _ = osRelease.Close() }()

	s := bufio.NewScanner(osRelease)
	s.Split(bufio.ScanLines)

	for s.Scan() {
		key, value, found := strings.Cut(s.Text(), "=")
		if !found {
			continue
		}
		values[key] = value
	}

	return values, nil
}
