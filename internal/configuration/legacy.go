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

type LegacyConfiguration struct {
	Includes      []string
	ConfigDirname string

	// Builder is used to build the legacy system. They must have Nix installed.
	Builder system.CommandRunner
}

func FindLegacyConfiguration(log logger.Logger, includes []string) (*LegacyConfiguration, error) {
	log.Debugf("looking for legacy configuration")

	var configuration string
	if nixosCfg, set := os.LookupEnv("NIXOS_CONFIG"); set {
		log.Debugf("$NIXOS_CONFIG is set, using automatically")
		configuration = nixosCfg
	}

	if configuration == "" && includes != nil {
		for _, include := range includes {
			if strings.HasPrefix(include, "nixos-config=") {
				configuration = strings.TrimPrefix(include, "nixos-config=")
				break
			}
		}
	}

	if configuration == "" {
		log.Debugf("$NIXOS_CONFIG not set, using $NIX_PATH to find configuration")

		nixPath := strings.Split(os.Getenv("NIX_PATH"), ":")
		for _, entry := range nixPath {
			if strings.HasPrefix(entry, "nixos-config=") {
				configuration = strings.TrimPrefix(entry, "nixos-config=")
				break
			}
		}

		if configuration == "" {
			return nil, fmt.Errorf("expected 'nixos-config' attribute to exist in NIX_PATH")
		}
	}

	configFileStat, err := os.Stat(configuration)
	if err != nil {
		return nil, err
	}

	if configFileStat.IsDir() {
		defaultNix := filepath.Join(configuration, "default.nix")

		info, err := os.Stat(defaultNix)
		if err != nil {
			return nil, err
		}

		if info.IsDir() {
			return nil, fmt.Errorf("%v is a directory, not a file", defaultNix)
		}
	} else {
		configuration = filepath.Dir(configuration)
	}

	return &LegacyConfiguration{
		Includes:      includes,
		ConfigDirname: configuration,
	}, nil
}

func (l *LegacyConfiguration) SetBuilder(builder system.CommandRunner) {
	l.Builder = builder
}

func (l *LegacyConfiguration) EvalAttribute(attr string) (*string, error) {
	configAttr := fmt.Sprintf("config.%s", attr)
	argv := []string{"nix-instantiate", "--eval", "<nixpkgs/nixos>", "-A", configAttr}

	for _, v := range l.Includes {
		argv = append(argv, "-I", v)
	}

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

func (l *LegacyConfiguration) buildLocalSystem(s *system.LocalSystem, buildType SystemBuildType, opts *SystemBuildOptions) (string, error) {
	nixCommand := "nix-build"
	if opts.UseNom {
		nixCommand = "nom-build"
	}

	argv := []string{nixCommand, "<nixpkgs/nixos>", "-A", buildType.BuildAttr()}

	// Mimic `nixos-rebuild` behavior of using -k option
	// for all commands except for switch and boot
	if buildType != SystemBuildTypeSystemActivation {
		argv = append(argv, "-k")
	}

	if opts.NixOpts != nil {
		argv = append(argv, nixopts.NixOptionsToArgsList(opts.CmdFlags, opts.NixOpts)...)
	}

	if opts.ResultLocation != "" {
		argv = append(argv, "--out-link", opts.ResultLocation)
	} else {
		argv = append(argv, "--no-out-link")
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

	_, err := l.Builder.Run(cmd)

	return strings.TrimSpace(stdout.String()), err
}

func (l *LegacyConfiguration) buildRemoteSystem(s *system.SSHSystem, buildType SystemBuildType, opts *SystemBuildOptions) (string, error) {
	log := s.Logger()

	localSystem := system.NewLocalSystem(log)

	extraBuildFlags := nixopts.NixOptionsToArgsListByCategory(opts.CmdFlags, opts.NixOpts, "build")

	// 1. Determine the drv path.
	// Equivalent of `nix-instantiate -A "${attr}" ${extraBuildFlags[@]}`
	instantiateArgv := []string{"nix-instantiate", "<nixpkgs/nixos>", "-A", buildType.BuildAttr()}
	instantiateArgv = append(instantiateArgv, extraBuildFlags...)

	var drvPathBuf bytes.Buffer
	instantiateCmd := system.NewCommand(instantiateArgv[0], instantiateArgv[1:]...)
	instantiateCmd.Stdout = &drvPathBuf

	if opts.GenerationTag != "" {
		instantiateCmd.SetEnv("NIXOS_GENERATION_TAG", opts.GenerationTag)
	}

	log.CmdArray(instantiateArgv)

	if _, err := localSystem.Run(instantiateCmd); err != nil {
		return "", fmt.Errorf("failed to instantiate configuration: %v", err)
	}

	drvPath := strings.TrimSpace(drvPathBuf.String())

	// 2. Copy the drv path over to the builder.
	// $ nix-copy-closure --to "$buildHost" "$drv"
	if err := system.CopyClosures(localSystem, s, []string{drvPath}); err != nil {
		return "", fmt.Errorf("failed to copy drv to build host: %v", err)
	}

	// 3. Realise the copied drv on the builder.
	// $ nix-store -r "$drv" "${buildArgs[@]}"
	realiseArgv := []string{"nix-store", "-r", drvPath}

	realiseNixOptions := nixopts.NixOptionsToArgsList(opts.CmdFlags, opts.NixOpts)
	realiseArgv = append(realiseArgv, realiseNixOptions...)

	// Mimic `nixos-rebuild` behavior of using -k option
	// for all commands except for switch and boot
	if buildType != SystemBuildTypeSystemActivation {
		realiseArgv = append(realiseArgv, "-k")
	}

	log.CmdArray(realiseArgv)

	var realisedPathBuf bytes.Buffer
	realiseDrvCmd := system.NewCommand(realiseArgv[0], realiseArgv[1:]...)
	realiseDrvCmd.Stdout = &realisedPathBuf

	_, err := s.Run(realiseDrvCmd)
	return strings.TrimSpace(realisedPathBuf.String()), err
}

func (l *LegacyConfiguration) BuildSystem(buildType SystemBuildType, opts *SystemBuildOptions) (string, error) {
	if l.Builder == nil {
		panic("LegacyConfiguration.Builder is nil")
	}

	switch s := l.Builder.(type) {
	case *system.SSHSystem:
		return l.buildRemoteSystem(s, buildType, opts)
	case *system.LocalSystem:
		return l.buildLocalSystem(s, buildType, opts)
	default:
		return "", fmt.Errorf("building is not implemented for this system type")
	}
}
