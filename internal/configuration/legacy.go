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
	"github.com/nix-community/nixos-cli/internal/nix"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/nix-community/nixos-cli/internal/utils"
)

type LegacyConfiguration struct {
	// Detected path to configuration. Only supports files
	// for now, not all kinds of Nix resources.
	ConfigPath string
	// Attribute to select from a Nix file when building.
	// Only relevant when UseExplicitPath is enabled.
	Attribute string
	// Do not use the implicit <nixpkgs/nixos> variable to
	// build. Pass the ConfigPath directly.
	UseExplicitPath bool
	// Extra entries to add to the NIX_PATH when invoking Nix.
	Includes []string

	// Builder is used to build the legacy system. They must have Nix installed.
	Builder system.CommandRunner
}

func FindLegacyConfiguration(log logger.Logger, includes []string) (*LegacyConfiguration, error) {
	log.Debugf("looking for legacy configuration")

	var configPath string
	if nixosCfg, set := os.LookupEnv("NIXOS_CONFIG"); set {
		log.Debugf("$NIXOS_CONFIG is set, using automatically")
		configPath = nixosCfg
	}

	if configPath == "" && includes != nil {
		for _, include := range includes {
			if after, ok := strings.CutPrefix(include, "nixos-config="); ok {
				configPath = after
				break
			}
		}
	}

	if configPath == "" {
		log.Debugf("$NIXOS_CONFIG not set, using $NIX_PATH to find configuration")

		nixPath := strings.SplitSeq(os.Getenv("NIX_PATH"), ":")
		for entry := range nixPath {
			if after, ok := strings.CutPrefix(entry, "nixos-config="); ok {
				configPath = after
				break
			}
		}
	}

	if configPath == "" {
		log.Debugf("no 'nixos-config' attribute exists in NIX_PATH")
		return nil, fmt.Errorf("no configuration found")
	}

	resolvedPath, err := utils.ResolveNixFilename(configPath)
	if err != nil {
		return nil, err
	}

	return &LegacyConfiguration{
		Includes:   includes,
		ConfigPath: resolvedPath,
	}, nil
}

// Get the directory that this configuration file is found in
func (l *LegacyConfiguration) Dirname() string {
	return filepath.Dir(l.ConfigPath)
}

func (l *LegacyConfiguration) SetBuilder(builder system.CommandRunner) {
	l.Builder = builder
}

func (l *LegacyConfiguration) ConfigPathArg() string {
	if l.UseExplicitPath {
		return l.ConfigPath
	}
	return "<nixpkgs/nixos>"
}

func (l *LegacyConfiguration) ConfigAttr(attrs ...string) string {
	toplevelAttr := ""
	if l.UseExplicitPath {
		toplevelAttr = l.Attribute
	}
	attrs = append([]string{toplevelAttr, "config"}, attrs...)
	return nix.MakeAttrPath(attrs...)
}

func (l *LegacyConfiguration) BuildAttr(attrs ...string) string {
	attrs = append([]string{"system", "build"}, attrs...)
	return l.ConfigAttr(attrs...)
}

func (l *LegacyConfiguration) EvalAttribute(attr string) (*string, error) {
	argv := []string{"nix-instantiate", "--eval", l.ConfigPathArg(), "-A", l.ConfigAttr(attr)}

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

func (l *LegacyConfiguration) buildLocalSystem(s *system.LocalSystem, buildType BuildType, opts *SystemBuildOptions) (string, error) {
	nixCommand := "nix-build"
	if opts.UseNom {
		nixCommand = "nom-build"
	}

	argv := []string{nixCommand, l.ConfigPathArg(), "-A", l.BuildAttr(buildType.BuildAttr())}

	// Mimic `nixos-rebuild` behavior of using -k option
	// for all commands except for `switch` and `boot`
	if v, ok := buildType.(*SystemBuild); !ok || !v.Activate {
		argv = append(argv, "-k")
	}

	if opts.NixOpts != nil {
		argv = append(argv, opts.NixOpts.ArgsForCommand(nixopts.CmdLegacyBuild)...)
	}

	if opts.ResultLocation != "" {
		argv = append(argv, "--out-link", opts.ResultLocation)
	} else {
		argv = append(argv, "--no-out-link")
	}

	if opts.DryBuild {
		argv = append(argv, "--dry-run")
	}

	if opts.ExtraArgs != nil {
		argv = append(argv, opts.ExtraArgs...)
	}

	log := s.Logger()

	if log.GetLogLevel() == logger.LogLevelDebug {
		argv = append(argv, "-v")
	}

	log.CmdArray(argv)

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

func (l *LegacyConfiguration) buildRemoteSystem(s *system.SSHSystem, buildType BuildType, opts *SystemBuildOptions) (string, error) {
	log := s.Logger()

	localSystem := system.NewLocalSystem(log)

	var extraInstantiateArgs []string
	var extraRealiseArgs []string
	if opts.NixOpts != nil {
		extraInstantiateArgs = opts.NixOpts.ArgsForCommand(nixopts.CmdInstantiate)
		extraRealiseArgs = opts.NixOpts.ArgsForCommand(nixopts.CmdStoreRealise)
	}

	// 1. Determine the drv path.
	// Equivalent of `nix-instantiate -A "${attr}" ${extraBuildFlags[@]}`
	instantiateArgv := []string{"nix-instantiate", "--no-gc-warning", l.ConfigPathArg(), "-A", l.BuildAttr(buildType.BuildAttr())}
	instantiateArgv = append(instantiateArgv, extraInstantiateArgs...)

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
	realiseArgv := []string{"nix-store", "--no-gc-warning", "-r", drvPath}
	realiseArgv = append(realiseArgv, extraRealiseArgs...)

	// Mimic `nixos-rebuild` behavior of using -k option
	// for all commands except for `switch` and `boot`
	if v, ok := buildType.(*SystemBuild); !ok || !v.Activate {
		realiseArgv = append(realiseArgv, "-k")
	}

	if opts.ResultLocation != "" {
		realiseArgv = append(realiseArgv, "--add-root", opts.ResultLocation)
	}

	if opts.DryBuild {
		realiseArgv = append(realiseArgv, "--dry-run")
	}

	log.CmdArray(realiseArgv)

	var realisedPathBuf bytes.Buffer
	realiseDrvCmd := system.NewCommand(realiseArgv[0], realiseArgv[1:]...)
	realiseDrvCmd.Stdout = &realisedPathBuf

	_, err := s.Run(realiseDrvCmd)
	if err != nil {
		return "", err
	}

	resultLocation := strings.TrimSpace(realisedPathBuf.String())
	if opts.ResultLocation != "" {
		resultLocation, err = s.FS().ReadLink(resultLocation)
		if err != nil {
			return "", fmt.Errorf("failed to resolve result location: %v", err)
		}
	}

	return resultLocation, err
}

func (l *LegacyConfiguration) BuildSystem(buildType BuildType, opts *SystemBuildOptions) (string, error) {
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
