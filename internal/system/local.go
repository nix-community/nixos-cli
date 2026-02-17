package system

import (
	"bufio"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

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

func (l *LocalSystem) FS() Filesystem {
	return &LocalFilesystem{}
}

func (l *LocalSystem) Run(cmd *Command) (int, error) {
	var commandName string
	var args []string

	if cmd.RootElevationCmd != "" {
		commandName = cmd.RootElevationCmd
		args = append([]string{cmd.Name}, cmd.Args...)
	} else {
		commandName = cmd.Name
		args = cmd.Args
	}

	command := exec.Command(commandName, args...)

	command.Stdout = cmd.Stdout
	command.Stderr = cmd.Stderr
	command.Stdin = cmd.Stdin
	command.Env = os.Environ()

	for key, value := range cmd.Env {
		command.Env = append(command.Env, key+"="+value)
	}

	if err := command.Start(); err != nil {
		return 0, err
	}

	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	defer func() {
		signal.Stop(sigs)
		close(sigs)
	}()

	go func() {
		for sig := range sigs {
			if command.Process != nil {
				if err := command.Process.Signal(sig); err != nil {
					l.logger.Warnf("failed to forward signal '%v': %v", sig, err)
				}
			}
		}
	}()

	err := command.Wait()

	if exitErr, ok := err.(*exec.ExitError); ok {
		type exitStatusImpl interface{ ExitStatus() int }

		var status exitStatusImpl
		if status, ok = exitErr.Sys().(exitStatusImpl); ok {
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

	osReleaseFile, err := os.Open("/etc/os-release")
	if err != nil {
		return false
	}
	defer func() { _ = osReleaseFile.Close() }()

	osRelease, err := parseOSRelease(osReleaseFile)
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

func (l *LocalSystem) IsRemote() bool {
	return false
}

func (l *LocalSystem) HasCommand(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func parseOSRelease(r io.Reader) (map[string]string, error) {
	values := make(map[string]string)

	s := bufio.NewScanner(r)
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
