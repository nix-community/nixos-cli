package system

import (
	"io"
	"maps"
	"os"
	"slices"

	"github.com/nix-community/nixos-cli/internal/logger"
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
