package system

import (
	"context"
	"errors"
	"fmt"

	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/nix-community/nixos-cli/internal/utils"
)

type RootElevator struct {
	Command          string
	Flags            []string
	Method           settings.PasswordInputMethod
	PasswordProvider PasswordProvider
}

type PasswordProvider interface {
	GetPassword() (string, error)
	PromptForPassword(ctx context.Context, prompt string) (string, error)
}

func ElevatorFromConfig(cfg *settings.RootCommandSettings) (*RootElevator, error) {
	var flags []string
	var provider PasswordProvider

	switch cfg.Command {
	case "sudo":
		switch cfg.PasswordMethod {
		case settings.PasswordInputMethodStdin:
			flags = append(flags, "-S", "-p", "")
		case settings.PasswordInputMethodNone:
			flags = append(flags, "-n")
		}
	case "doas":
		switch cfg.PasswordMethod {
		case settings.PasswordInputMethodStdin:
			return nil, errors.New("doas does not support password input through stdin")
		case settings.PasswordInputMethodNone:
			flags = append(flags, "-n")
		}
	case "run0":
		switch cfg.PasswordMethod {
		case settings.PasswordInputMethodNone, settings.PasswordInputMethodStdin:
			return nil, errors.New("run0 only supports the tty method for password input")
		}
	default:
		flags = nil
	}

	if cfg.PasswordMethod == settings.PasswordInputMethodStdin {
		provider = &CachedPasswordProvider{}
	}

	return &RootElevator{
		Command:          cfg.Command,
		Flags:            flags,
		Method:           cfg.PasswordMethod,
		PasswordProvider: provider,
	}, nil
}

func (e *RootElevator) PromptIfNecessary(ctx context.Context) error {
	if e.PasswordProvider == nil {
		return nil
	}

	prompt := fmt.Sprintf("[%s] enter password: ", e.Command)

	_, err := e.PasswordProvider.PromptForPassword(ctx, prompt)
	return err
}

type CachedPasswordProvider struct {
	password string
	set      bool
}

func (p *CachedPasswordProvider) GetPassword() (string, error) {
	if p.set {
		return p.password, nil
	}

	return "", errors.New("root elevation password not set")
}

func (p *CachedPasswordProvider) PromptForPassword(ctx context.Context, prompt string) (string, error) {
	if p.set {
		return p.password, nil
	}

	password, err := utils.PromptForPassword(ctx, prompt)
	if err != nil {
		return "", err
	}

	p.password = string(password)
	p.set = true
	return p.password, nil
}
