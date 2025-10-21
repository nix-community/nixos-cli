package configuration

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/nix-community/nixos-cli/internal/cmd/nixopts"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/system"
)

type FlakeRef struct {
	URI    string
	System string

	// Builder is used to build the flake ref. They must have Nix installed.
	Builder system.CommandRunner
}

func FlakeRefFromString(s string) *FlakeRef {
	split := strings.Index(s, "#")

	var uri string
	if split > -1 {
		uri = s[:split]
	} else {
		uri = s
	}

	if _, err := os.Stat(uri); err == nil {
		if resolved, err := filepath.EvalSymlinks(uri); err == nil {
			uri = resolved
		}
	}

	if split > -1 {
		return &FlakeRef{
			URI:    uri,
			System: s[split+1:],
		}
	}

	return &FlakeRef{
		URI:    uri,
		System: "",
	}
}

func FlakeRefFromEnv(defaultLocation string) (*FlakeRef, error) {
	nixosConfig, set := os.LookupEnv("NIXOS_CONFIG")
	if !set {
		nixosConfig = defaultLocation
	}

	if nixosConfig == "" {
		return nil, fmt.Errorf("NIXOS_CONFIG is not set")
	}

	return FlakeRefFromString(nixosConfig), nil
}

func (f *FlakeRef) InferSystemFromHostnameIfNeeded() error {
	if f.System == "" {
		hostname, err := os.Hostname()
		if err != nil {
			return err
		}

		f.System = hostname
	}

	return nil
}

func (f *FlakeRef) SetBuilder(builder system.CommandRunner) {
	f.Builder = builder
}

func (f *FlakeRef) EvalAttribute(attr string) (*string, error) {
	evalArg := fmt.Sprintf(`%s#nixosConfigurations.%s.config.%s`, f.URI, f.System, attr)
	argv := []string{"nix", "eval", evalArg}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := exec.Command(argv[0], argv[1:]...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return nil, &AttributeEvaluationError{
			Attribute:        attr,
			EvaluationOutput: strings.TrimSpace(stderr.String()),
		}
	}

	value := strings.TrimSpace(stdout.String())

	return &value, nil
}

func (f *FlakeRef) BuildSystem(buildType SystemBuildType, opts *SystemBuildOptions) (string, error) {
	if f.Builder == nil {
		panic("FlakeRef.Builder is nil")
	}

	nixCommand := "nix"
	if opts.UseNom {
		nixCommand = "nom"
	}

	systemAttribute := fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.%s", f.URI, f.System, buildType.BuildAttr())

	argv := []string{nixCommand, "build", systemAttribute, "--print-out-paths"}

	if opts.ResultLocation != "" {
		argv = append(argv, "--out-link", opts.ResultLocation)
	} else {
		argv = append(argv, "--no-link")
	}

	if opts.DryBuild {
		argv = append(argv, "--dry-run")
	}

	if opts.NixOpts != nil {
		argv = append(argv, nixopts.NixOptionsToArgsList(opts.CmdFlags, opts.NixOpts)...)
	}

	if opts.ExtraArgs != nil {
		argv = append(argv, opts.ExtraArgs...)
	}

	log := f.Builder.Logger()
	if log.GetLogLevel() == logger.LogLevelDebug {
		argv = append(argv, "-v")
	}
	f.Builder.Logger().CmdArray(argv)

	var stdout bytes.Buffer
	cmd := system.NewCommand(nixCommand, argv[1:]...)
	cmd.Stdout = &stdout

	if opts.GenerationTag != "" {
		cmd.SetEnv("NIXOS_GENERATION_TAG", opts.GenerationTag)
	}

	for k, v := range opts.Env {
		cmd.SetEnv(k, v)
	}

	_, err := f.Builder.Run(cmd)

	return strings.Trim(stdout.String(), "\n "), err
}
