package configuration

import (
	"fmt"

	"github.com/nix-community/nixos-cli/internal/build"
	"github.com/nix-community/nixos-cli/internal/cmd/nixopts"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/nix"
	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/spf13/pflag"
)

type SystemBuildOptions struct {
	ResultLocation string
	DryBuild       bool
	UseNom         bool
	GenerationTag  string

	// Command-line flags that were passed for the command context.
	// This is needed to determine the proper Nix options to pass
	// when building, if any were passed through.
	CmdFlags  *pflag.FlagSet
	NixOpts   nixopts.NixOptionsSet
	Env       map[string]string
	ExtraArgs []string
}

type SystemEvalOptions struct {
	NixOpts nixopts.NixOptionsSet
}

type Configuration interface {
	SetBuilder(builder system.CommandRunner)
	EvalAttribute(attr string) (*string, error)
	EvalSystem(s *system.LocalSystem, buildType BuildType, opts *SystemEvalOptions) (string, error)
	BuildSystem(buildType BuildType, opts *SystemBuildOptions) (string, error)
}

type AttributeEvaluationError struct {
	Attribute        string
	EvaluationOutput string
}

func (e *AttributeEvaluationError) Error() string {
	return fmt.Sprintf("failed to evaluate attribute %s, trace:\n%s", e.Attribute, e.EvaluationOutput)
}

func FindConfiguration(log logger.Logger, cfg *settings.Settings, includes []string) (Configuration, error) {
	if build.Flake() {
		log.Debug("looking for flake configuration")

		f, err := FlakeRefFromEnv(cfg.ConfigLocation)
		if err != nil {
			return nil, err
		}

		if err = f.InferSystemFromHostnameIfNeeded(); err != nil {
			return nil, err
		}

		log.Debugf("found flake configuration: %s#%s", f.URI, f.System)

		return f, nil
	} else {
		c, err := FindLegacyConfiguration(log, includes)
		if err != nil {
			return nil, err
		}

		log.Debugf("found legacy configuration at %s", c.ConfigPath)

		return c, nil
	}
}

type BuildType interface {
	BuildAttr() string
}

type SystemBuild struct {
	Activate bool
}

func (s *SystemBuild) BuildAttr() string {
	return "toplevel"
}

type VMBuild struct {
	WithBootloader bool
}

func (v *VMBuild) BuildAttr() string {
	if v.WithBootloader {
		return "vmWithBootLoader"
	} else {
		return "vm"
	}
}

type ImageBuild struct {
	Variant string
}

func (i *ImageBuild) BuildAttr() string {
	return nix.MakeAttrPath("images", nix.MakeAttrName(i.Variant))
}
