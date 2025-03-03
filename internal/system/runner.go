package system

import (
	"io"
	"os"
)

type CommandRunner interface {
	Run(cmd *Command) (int, error)
	LogCmd(argv []string)
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
