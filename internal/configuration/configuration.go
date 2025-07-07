package configuration

import (
	"fmt"

	"github.com/nix-community/nixos-cli/internal/build"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/spf13/pflag"
)

type SystemBuildOptions struct {
	ResultLocation string
	DryBuild       bool
	UseNom         bool
	GenerationTag  string
	Verbose        bool

	// Command-line flags that were passed for the command context.
	// This is needed to determine the proper Nix options to pass
	// when building, if any were passed through.
	CmdFlags  *pflag.FlagSet
	NixOpts   any
	Env       map[string]string
	ExtraArgs []string
}

type Configuration interface {
	SetBuilder(builder system.CommandRunner)
	EvalAttribute(attr string) (*string, error)
	BuildSystem(buildType SystemBuildType, opts *SystemBuildOptions) (string, error)
}

type AttributeEvaluationError struct {
	Attribute        string
	EvaluationOutput string
}

func (e *AttributeEvaluationError) Error() string {
	return fmt.Sprintf("failed to evaluate attribute %s", e.Attribute)
}

func FindConfiguration(log *logger.Logger, cfg *settings.Settings, includes []string, verbose bool) (Configuration, error) {
	if build.Flake() {
		if verbose {
			log.Info("looking for flake configuration")
		}

		f, err := FlakeRefFromEnv(cfg.ConfigLocation)
		if err != nil {
			return nil, err
		}

		if err := f.InferSystemFromHostnameIfNeeded(); err != nil {
			return nil, err
		}

		if verbose {
			log.Infof("found flake configuration: %s#%s", f.URI, f.System)
		}

		return f, nil
	} else {
		c, err := FindLegacyConfiguration(log, includes, verbose)
		if err != nil {
			return nil, err
		}

		if verbose {
			log.Infof("found legacy configuration at %s", c)
		}

		return c, nil
	}
}

type SystemBuildType int

const (
	SystemBuildTypeSystem SystemBuildType = iota
	SystemBuildTypeSystemActivation
	SystemBuildTypeVM
	SystemBuildTypeVMWithBootloader
)

func (b SystemBuildType) BuildAttr() string {
	switch b {
	case SystemBuildTypeSystem, SystemBuildTypeSystemActivation:
		if build.Flake() {
			return "toplevel"
		} else {
			return "system"
		}
	case SystemBuildTypeVM:
		return "vm"
	case SystemBuildTypeVMWithBootloader:
		return "vmWithBootLoader"
	default:
		panic("unknown build type")
	}
}

func (b SystemBuildType) IsVM() bool {
	return b == SystemBuildTypeVM || b == SystemBuildTypeVMWithBootloader
}

func (b SystemBuildType) IsSystem() bool {
	return b == SystemBuildTypeSystem || b == SystemBuildTypeSystemActivation
}
