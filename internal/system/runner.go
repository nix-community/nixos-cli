package system

import (
	"errors"
	"io"
	"maps"
	"os"
	"slices"
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

	RootElevationCmd      string
	RootElevationCmdFlags []string
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
	c.Env[key] = value
}

func (c *Command) AsRoot(rootCmd string, rootCmdFlags ...string) *Command {
	c.RootElevationCmd = rootCmd
	c.RootElevationCmdFlags = rootCmdFlags
	return c
}

func (c *Command) Clone() *Command {
	args := slices.Clone(c.Args)

	env := make(map[string]string, len(c.Env))
	maps.Copy(env, c.Env)

	rootElevationCmdFlags := slices.Clone(c.RootElevationCmdFlags)

	return &Command{
		Name:   c.Name,
		Args:   args,
		Stdin:  c.Stdin,
		Stdout: c.Stdout,
		Stderr: c.Stderr,
		Env:    env,

		RootElevationCmd:      c.RootElevationCmd,
		RootElevationCmdFlags: rootElevationCmdFlags,
	}
}

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

	if strings.IndexByte(c.RootElevationCmd, 0) != -1 {
		return nil, errors.New("NUL (0x00) bytes are not allowed in env values or args")
	}

	for _, a := range c.Args {
		if strings.IndexByte(a, 0) != -1 {
			return nil, errors.New("NUL (0x00) bytes are not allowed in env values or args")
		}
	}

	for _, a := range c.RootElevationCmdFlags {
		if strings.IndexByte(a, 0) != -1 {
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
	if c.RootElevationCmd != "" {
		// Root elevation commands/flags must go BEFORE the `sh` invocation.
		// This will ensure that the environment is preserved across the
		// elevation boundary.
		argv = append([]string{c.RootElevationCmd}, c.RootElevationCmdFlags...)
	}
	argv = append(argv, "sh", "-c", wrappedCmdScript)

	return argv, nil
}
