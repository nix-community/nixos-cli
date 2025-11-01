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
	Name           string
	Args           []string
	Stdin          io.Reader
	Stdout         io.Writer
	Stderr         io.Writer
	Env            map[string]string
	ForwardSignals bool
}

func NewCommand(name string, args ...string) *Command {
	return &Command{
		Name:           name,
		Args:           args,
		Stdin:          os.Stdin,
		Stdout:         os.Stdout,
		Stderr:         os.Stderr,
		Env:            make(map[string]string),
		ForwardSignals: true,
	}
}

func (c *Command) SetEnv(key string, value string) {
	c.Env[key] = value
}

func (c *Command) RunAsRoot(rootCmd string) *Command {
	if rootCmd == "" {
		return c
	}

	newArgs := append([]string{c.Name}, c.Args...)
	c.Name = rootCmd
	c.Args = newArgs

	return c
}
