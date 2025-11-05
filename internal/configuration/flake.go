package configuration

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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

func (f *FlakeRef) buildLocalSystem(s *system.LocalSystem, buildType BuildType, opts *SystemBuildOptions) (string, error) {
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

	log := s.Logger()
	if log.GetLogLevel() == logger.LogLevelDebug {
		argv = append(argv, "-v")
	}

	s.Logger().CmdArray(argv)

	var stdout bytes.Buffer
	cmd := system.NewCommand(nixCommand, argv[1:]...)
	cmd.Stdout = &stdout

	if opts.GenerationTag != "" {
		cmd.SetEnv("NIXOS_GENERATION_TAG", opts.GenerationTag)
	}

	for k, v := range opts.Env {
		cmd.SetEnv(k, v)
	}

	_, err := s.Run(cmd)

	return strings.Trim(stdout.String(), "\n "), err
}

func (f *FlakeRef) buildRemoteSystem(s *system.SSHSystem, buildType BuildType, opts *SystemBuildOptions) (string, error) {
	evalArgs := nixopts.NixOptionsToArgsListByCategory(opts.CmdFlags, opts.NixOpts, "lock")
	buildArgs := nixopts.NixOptionsToArgsListByCategory(opts.CmdFlags, opts.NixOpts, "build")

	// --impure must be part of the eval arguments, rather than
	// the build arguments here, since that is where environment
	// variables are accessed.
	if i := slices.Index(buildArgs, "--impure"); i >= 0 {
		evalArgs = append(evalArgs, "--impure")
		buildArgs = slices.Delete(buildArgs, i, i+1)
	}

	log := s.Logger()

	localSystem := system.NewLocalSystem(log)

	// 1. Determine the drv path.
	// Equivalent of `nix eval --raw "${attr}.drvPath"`
	drvPathAttr := fmt.Sprintf("%s#nixosConfigurations.%s.config.system.build.%s.drvPath", f.URI, f.System, buildType.BuildAttr())

	evalDrvCmdArgv := []string{"nix", "eval", "--raw", drvPathAttr}
	evalDrvCmdArgv = append(evalDrvCmdArgv, evalArgs...)

	var drvPathBuf bytes.Buffer
	evalDrvCmd := system.NewCommand(evalDrvCmdArgv[0], evalDrvCmdArgv[1:]...)
	evalDrvCmd.Stdout = &drvPathBuf

	if opts.GenerationTag != "" {
		evalDrvCmd.SetEnv("NIXOS_GENERATION_TAG", opts.GenerationTag)
	}

	log.CmdArray(evalDrvCmdArgv)

	_, err := localSystem.Run(evalDrvCmd)
	if err != nil {
		return "", fmt.Errorf("failed to evaluate drvPath attribute for configuration: %v", err)
	}

	drvPath := strings.TrimSpace(drvPathBuf.String())

	// 2. Copy the drv path over to the builder.
	// $ nix "${flakeFlags[@]}" copy "${copyFlags[@]}" --derivation --to "ssh://$buildHost" "$drv"

	copyFlags := nixopts.NixOptionsToArgsListByCategory(opts.CmdFlags, opts.NixOpts, "copy")

	if err = system.CopyClosures(localSystem, s, []string{drvPath}, copyFlags...); err != nil {
		return "", fmt.Errorf("failed to copy drv to build host: %v", err)
	}

	// 3. Realise the copied drv on the builder.
	// $ nix-store -r "$drv" "${buildArgs[@]}"

	realiseDrvArgv := []string{"nix-store", "-r", drvPath}
	realiseDrvArgv = append(realiseDrvArgv, buildArgs...)

	log.CmdArray(realiseDrvArgv)

	var realisedPathBuf bytes.Buffer
	realiseDrvCmd := system.NewCommand(realiseDrvArgv[0], realiseDrvArgv[1:]...)
	realiseDrvCmd.Stdout = &realisedPathBuf

	_, err = s.Run(realiseDrvCmd)

	return strings.TrimSpace(realisedPathBuf.String()), err
}

func (f *FlakeRef) BuildSystem(buildType BuildType, opts *SystemBuildOptions) (string, error) {
	if f.Builder == nil {
		panic("FlakeRef.Builder is nil")
	}

	switch s := f.Builder.(type) {
	case *system.SSHSystem:
		return f.buildRemoteSystem(s, buildType, opts)
	case *system.LocalSystem:
		return f.buildLocalSystem(s, buildType, opts)
	default:
		return "", fmt.Errorf("building is not implemented for this system type")
	}
}
