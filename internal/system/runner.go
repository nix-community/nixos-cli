package system

import (
	"io"
	"os"

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
