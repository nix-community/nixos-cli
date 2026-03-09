package system

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"regexp"
	"strings"
	"syscall"

	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
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
	var args []string

	var err error

	if cmd.rootElevator != nil {
		// If environment variables are specified,
		// use the shell wrapper to set them inline
		// to bypass root elevation environment
		// variable sanitizing.
		if len(cmd.Env) > 0 {
			args, err = cmd.BuildShellWrapper()
			if err != nil {
				return 0, err
			}
		} else {
			args = cmd.BuildArgs()
		}
	} else {
		args = cmd.BuildArgs()
	}

	if len(args) == 0 {
		return 0, fmt.Errorf("command has empty argument list")
	}

	command := exec.Command(args[0], args[1:]...)

	command.Stdout = cmd.Stdout
	command.Stderr = cmd.Stderr
	command.Stdin = cmd.Stdin
	command.Env = os.Environ()

	if elevator := cmd.rootElevator; elevator != nil {
		switch elevator.Method {
		case settings.PasswordInputMethodStdin:
			if elevator.PasswordProvider == nil {
				return 0, fmt.Errorf("password provider is required for stdin password input method")
			}
			// Processes will likely never expect stdin to be set for SSH
			// if they are running as root, since this seems to be a
			// fairly uncommon scenario to need to pass things through
			// stdin while simultaneously needing root, and we will likely
			// never need something like that here.
			//
			// As such, we're replacing the entire stdin with this password.
			pwStr, pwErr := elevator.PasswordProvider.GetPassword()
			if pwErr != nil {
				return 0, pwErr
			}
			pw := append([]byte(pwStr), '\n')
			command.Stdin = bytes.NewReader(pw)
		case settings.PasswordInputMethodTTY:
			// Do nothing; if the input is not a terminal, then
			// it will fail on its own.
		case settings.PasswordInputMethodNone:
			command.Stdin = nil
		default:
			return 0, fmt.Errorf("unsupported password input method: %v", elevator.Method)
		}
	} else {
		// If the root elevator is set, then these variables are
		// handled by the wrapper. Otherwise, set them directly
		// for the process.
		for key, value := range cmd.Env {
			command.Env = append(command.Env, key+"="+value)
		}
	}

	if err = command.Start(); err != nil {
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
				if signalErr := command.Process.Signal(sig); signalErr != nil {
					l.logger.Warnf("failed to forward signal '%v': %v", sig, signalErr)
				}
			}
		}
	}()

	err = command.Wait()

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
