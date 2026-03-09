package system

import (
	"errors"
	"io"
	"os"
	"regexp"
	"sort"
	"strings"

	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/utils"
)

type CommandRunner interface {
	Run(cmd *Command) (int, error)
	Logger() logger.Logger
	HasCommand(cmd string) bool
}

type Command struct {
	Name   string
	Args   []string
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
	Env    map[string]string

	rootElevator *RootElevator
}

func NewCommand(name string, args ...string) *Command {
	return &Command{
		Name:   name,
		Args:   args,
		Stdin:  os.Stdin,
		Stdout: os.Stdout,
		Stderr: os.Stderr,
		Env:    make(map[string]string),
	}
}

func (c *Command) SetEnv(key string, value string) {
	if c.Env == nil {
		c.Env = make(map[string]string)
	}

	c.Env[key] = value
}

func (c *Command) AsRoot(elevator *RootElevator) *Command {
	c.rootElevator = elevator
	return c
}

var envVarNamePattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// Build a safe `sh -c` wrapper that can support setting
// environment variables for a process inline.
//
// This is required for environments that sanitize environment
// variables, such as when commands are ran using `sudo` or other
// root elevation commands, or running commands on systems using
// SSH without proper AcceptEnv settings configured.
func (c *Command) BuildShellWrapper() ([]string, error) {
	// Make sure all passed arguments and env vars
	// do not have any NUL bytes and have valid names.
	if strings.IndexByte(c.Name, 0) != -1 {
		return nil, errors.New("NUL (0x00) bytes are not allowed in env values or args")
	}

	for _, a := range c.Args {
		if strings.IndexByte(a, 0) != -1 {
			return nil, errors.New("NUL (0x00) bytes are not allowed in env values or args")
		}
	}

	if elevator := c.rootElevator; elevator != nil {
		for _, a := range elevator.Flags {
			if strings.IndexByte(a, 0) != -1 {
				return nil, errors.New("NUL (0x00) bytes are not allowed in env values or args")
			}
		}

		if strings.IndexByte(elevator.Command, 0) != -1 {
			return nil, errors.New("NUL (0x00) bytes are not allowed in env values or args")
		}
	}

	for k, v := range c.Env {
		if !envVarNamePattern.MatchString(k) {
			return nil, errors.New("invalid env var name: " + k)
		}
		if strings.IndexByte(v, 0) != -1 {
			return nil, errors.New("NUL (0x00) bytes are not allowed in env values or args")
		}
	}

	// Use deterministic ordering for env exports,
	// by alphabetical order
	keys := make([]string, 0, len(c.Env))
	for k := range c.Env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Start building the command string; first, all variables
	// get exported inline.
	var b strings.Builder
	for _, k := range keys {
		q := utils.Quote(c.Env[k])
		b.WriteString("export ")
		b.WriteString(k)
		b.WriteByte('=')
		b.WriteString(q)
		b.WriteString("; ")
	}

	// Set positional parameters to the passed args,
	// and execute them inline using `exec`.
	b.WriteString("set -- ")
	b.WriteString(utils.Quote(c.Name))
	for _, a := range c.Args {
		q := utils.Quote(a)
		b.WriteByte(' ')
		b.WriteString(q)
	}
	b.WriteString(`; exec "$@"`)

	wrappedCmdScript := b.String()

	var argv []string
	if elevator := c.rootElevator; elevator != nil {
		// Root elevation commands/flags must go BEFORE the `sh` invocation.
		// This will ensure that the environment is preserved across the
		// elevation boundary.
		argv = append([]string{elevator.Command}, elevator.Flags...)
	}
	argv = append(argv, "sh", "-c", wrappedCmdScript)

	return argv, nil
}

// Build an arguments array, suitable for passing to exec.Command
// or other argument string slice parameters.
//
// This does not set environment variables; use BuildShellWrapper()
// for that or use cmd.Env directly at the call site for System.Run()
// implementations.
func (c *Command) BuildArgs() []string {
	var argv []string

	if elevator := c.rootElevator; elevator != nil {
		argv = append(argv, elevator.Command)
		argv = append(argv, elevator.Flags...)
	}

	argv = append(argv, c.Name)
	argv = append(argv, c.Args...)

	return argv
}

// Inherit the passed environment variables' values explicitly
// into the command's env map.
//
// Useful when passing variables over `sudo` or SSH environment
// variable sanitation barriers.
func (c *Command) InheritEnv(vars ...string) {
	if c.Env == nil {
		c.Env = make(map[string]string)
	}

	for _, name := range vars {
		if value, set := os.LookupEnv(name); set {
			c.Env[name] = value
		}
	}
}
