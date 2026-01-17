package install

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/nix-community/nixos-cli/internal/build"
	"github.com/nix-community/nixos-cli/internal/cmd/nixopts"
	"github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/configuration"
	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/nix-community/nixos-cli/internal/utils"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func InstallCommand() *cobra.Command {
	opts := cmdOpts.InstallOpts{}

	var usage string
	if build.Flake() {
		usage = "install {FLAKE-URI}#{SYSTEM-NAME}"
	} else {
		usage = "install [FILE] [ATTR]"
	}

	cmd := cobra.Command{
		Use:   usage,
		Short: "Install a NixOS system",
		Long:  "Install a NixOS system from a given configuration.",
		Args: func(cmd *cobra.Command, args []string) error {
			if build.Flake() {
				if err := cobra.ExactArgs(1)(cmd, args); err != nil {
					return err
				}

				ref := configuration.FlakeRefFromString(args[0])
				if ref.System == "" {
					return fmt.Errorf("missing required argument {SYSTEM-NAME}")
				}
				opts.FlakeRef = ref
			} else {
				if err := cobra.MaximumNArgs(2)(cmd, args); err != nil {
					return err
				}

				if len(args) > 0 {
					opts.File = args[0]
				}

				if len(args) > 1 {
					opts.Attr = args[1]
				}
			}

			if opts.Root != "" && !filepath.IsAbs(opts.Root) {
				return fmt.Errorf("--root must be an absolute path")
			}

			if opts.SystemClosure != "" {
				if !filepath.IsAbs(opts.SystemClosure) {
					return fmt.Errorf("--system must be an absolute path")
				}

				if opts.FlakeRef != nil {
					return fmt.Errorf("--system was specified, but [FLAKE-REF] was also provided; use one or the other")
				}

				if _, err := os.Stat(opts.SystemClosure); err != nil {
					return err
				}
			}

			return nil
		},
		ValidArgsFunction: cmdUtils.FlakeOrNixFileCompletions,
		PreRun: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			log := logger.FromContext(ctx)

			if opts.Verbose {
				log.SetLogLevel(logger.LogLevelDebug)
			}

			ctx = logger.WithLogger(ctx, log)
			cmd.SetContext(ctx)
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdUtils.CommandErrorHandler(installMain(cmd, &opts))
		},
	}

	cmd.Flags().StringVarP(&opts.Channel, "channel", "c", "", "Use derivation at `path` as the 'nixos' channel to copy")
	cmd.Flags().BoolVar(&opts.NoBootloader, "no-bootloader", false, "Do not install bootloader on device")
	cmd.Flags().BoolVar(&opts.NoChannelCopy, "no-channel-copy", false, "Do not copy over a NixOS channel")
	cmd.Flags().BoolVar(&opts.NoRootPassword, "no-root-passwd", false, "Do not prompt for setting root password")
	cmd.Flags().StringVarP(&opts.Root, "root", "r", "/mnt", "Treat `dir` as the root for installation")
	cmd.Flags().StringVarP(&opts.SystemClosure, "system", "s", "", "Install system from system closure at `path`")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Show verbose logging")

	nixopts.AddQuietNixOption(&cmd, &opts.NixOptions.Quiet)
	nixopts.AddPrintBuildLogsNixOption(&cmd, &opts.NixOptions.PrintBuildLogs)
	nixopts.AddNoBuildOutputNixOption(&cmd, &opts.NixOptions.NoBuildOutput)
	nixopts.AddShowTraceNixOption(&cmd, &opts.NixOptions.ShowTrace)
	nixopts.AddKeepGoingNixOption(&cmd, &opts.NixOptions.KeepGoing)
	nixopts.AddKeepFailedNixOption(&cmd, &opts.NixOptions.KeepFailed)
	nixopts.AddFallbackNixOption(&cmd, &opts.NixOptions.Fallback)
	nixopts.AddRefreshNixOption(&cmd, &opts.NixOptions.Refresh)
	nixopts.AddRepairNixOption(&cmd, &opts.NixOptions.Repair)
	nixopts.AddImpureNixOption(&cmd, &opts.NixOptions.Impure)
	nixopts.AddOfflineNixOption(&cmd, &opts.NixOptions.Offline)
	nixopts.AddNoNetNixOption(&cmd, &opts.NixOptions.NoNet)
	nixopts.AddMaxJobsNixOption(&cmd, &opts.NixOptions.MaxJobs)
	nixopts.AddCoresNixOption(&cmd, &opts.NixOptions.Cores)
	nixopts.AddLogFormatNixOption(&cmd, &opts.NixOptions.LogFormat)
	nixopts.AddOptionNixOption(&cmd, &opts.NixOptions.Options)
	nixopts.AddIncludesNixOption(&cmd, &opts.NixOptions.Includes)

	if build.Flake() {
		nixopts.AddRecreateLockFileNixOption(&cmd, &opts.NixOptions.RecreateLockFile)
		nixopts.AddNoUpdateLockFileNixOption(&cmd, &opts.NixOptions.NoUpdateLockFile)
		nixopts.AddNoWriteLockFileNixOption(&cmd, &opts.NixOptions.NoWriteLockFile)
		nixopts.AddNoUseRegistriesNixOption(&cmd, &opts.NixOptions.NoUseRegistries)
		nixopts.AddCommitLockFileNixOption(&cmd, &opts.NixOptions.CommitLockFile)
		nixopts.AddUpdateInputNixOption(&cmd, &opts.NixOptions.UpdateInputs)
		nixopts.AddOverrideInputNixOption(&cmd, &opts.NixOptions.OverrideInputs)
	}

	_ = cmd.RegisterFlagCompletionFunc("channel", cmdUtils.DirCompletions)
	_ = cmd.RegisterFlagCompletionFunc("root", cmdUtils.DirCompletions)
	_ = cmd.RegisterFlagCompletionFunc("system", cmdUtils.DirCompletions)

	cmd.MarkFlagsMutuallyExclusive("channel", "no-channel-copy")

	helpTemplate := cmd.HelpTemplate()
	if build.Flake() {
		helpTemplate += `
Arguments:
  [FLAKE-URI]    Flake URI that contains NixOS system to build
  [SYSTEM-NAME]  Name of NixOS system attribute to build
`
	} else {
		helpTemplate += `
Arguments:
  [FILE]  File to build configuration from
  [ATTR]  Attribute inside of [FILE] pointing to configuration

  Both arguments are optional. If [FILE] is not specified, then
  $root/etc/nixos/configuration.nix is used. If [ATTR] is not
  specified, then the top-level attribute of [FILE] is used.
`
	}
	helpTemplate += `
This command also forwards Nix options passed here to all relevant Nix invocations.
Check the Nix manual page for more details on what options are available.
`

	cmd.SetHelpTemplate(helpTemplate)
	cmdUtils.SetHelpFlagText(&cmd)

	return &cmd
}

func validateMountpoint(log logger.Logger, mountpoint string) error {
	stat, err := os.Stat(mountpoint)
	if err != nil {
		log.Errorf("failed to stat %v: %v", mountpoint, err)
		return err
	}

	if !stat.IsDir() {
		msg := fmt.Sprintf("mountpoint %v is not a directory", mountpoint)
		log.Error(msg)
		return fmt.Errorf("%v", msg)
	}

	// Check permissions for the mountpoint. All components in the
	// mountpoint directory must have an "other users" bit set to at
	// least 5 (read+execute).

	currentPath := "/"
	for _, component := range filepath.SplitList(mountpoint) {
		if component == "" {
			continue
		}

		currentPath = filepath.Join(currentPath, component)

		info, err := os.Stat(currentPath)
		if err != nil {
			return fmt.Errorf("failed to stat %s: %w", currentPath, err)
		}

		mode := info.Mode()
		hasCorrectPermission := mode.Perm()&0o005 >= 0o005

		if !hasCorrectPermission {
			msg := fmt.Sprintf("path %s should have permissions 755, but had permissions %s", currentPath, mode.Perm())
			log.Errorf(msg)
			log.Printf("hint: consider running `chmod o+rx %s", currentPath)

			return fmt.Errorf("%v", msg)
		}
	}

	return nil
}

const (
	defaultExtraSubstituters = "auto?trusted=1"
)

func copyChannel(cobraCmd *cobra.Command, s system.CommandRunner, mountpoint string, channelDirectory string, buildOptions any) error {
	log := s.Logger()

	mountpointChannelDir := filepath.Join(mountpoint, constants.NixChannelDirectory)
	rootProfileDir := filepath.Dir(mountpointChannelDir)

	err := os.MkdirAll(rootProfileDir, 0o755)
	if err != nil {
		return fmt.Errorf("failed to create %s: %s", rootProfileDir, err)
	}

	channelPath := channelDirectory
	if channelPath == "" {
		argv := []string{"nix-env", "-p", constants.NixChannelDirectory, "-q", "nixos", "--no-name", "--out-path"}

		var stdout bytes.Buffer

		cmd := system.NewCommand(argv[0], argv[1:]...)
		cmd.Stdout = &stdout

		_, err := s.Run(cmd)
		if err != nil {
			log.Errorf("failed to obtain default nixos channel location: %v", err)
			return err
		}

		channelPath = strings.TrimSpace(stdout.String())
	}

	argv := []string{"nix-env", "--store", mountpoint}
	argv = append(argv, nixopts.NixOptionsToArgsList(cobraCmd.Flags(), buildOptions)...)
	argv = append(argv, "--extra-substituters", defaultExtraSubstituters)
	argv = append(argv, "-p", mountpointChannelDir, "--set", channelPath)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	log.CmdArray(argv)

	_, err = s.Run(cmd)
	if err != nil {
		log.Errorf("failed to copy channel: %v", err)
		return err
	}

	defexprDirname := filepath.Join(mountpoint, "root", ".nix-defexpr")
	err = os.MkdirAll(defexprDirname, 0o700)
	if err != nil {
		log.Errorf("failed to create .nix-defexpr directory when copying channel: %v", err)
		return err
	}

	defexprChannelsDirname := filepath.Join(defexprDirname, "channels")
	err = os.RemoveAll(defexprChannelsDirname)
	if err != nil {
		log.Errorf("failed to remove .nix-defexpr/channels directory: %v", err)
		return err
	}

	err = os.Symlink(mountpointChannelDir, defexprChannelsDirname)
	if err != nil {
		log.Errorf("failed to create .nix-defexpr/channels symlink: %v", err)
		return err
	}

	return nil
}

func createInitialGeneration(s system.CommandRunner, mountpoint string, closure string) error {
	systemProfileDir := filepath.Join(mountpoint, constants.NixProfileDirectory, "system")

	log := s.Logger()

	if err := os.MkdirAll(filepath.Dir(systemProfileDir), 0o755); err != nil {
		log.Errorf("failed to create nix system profile directory for new system: %v", err)
		return err
	}

	argv := []string{
		"nix-env", "--store", mountpoint, "-p", systemProfileDir,
		"--set", closure, "--extra-substituters", defaultExtraSubstituters,
	}

	log.CmdArray(argv)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	_, err := s.Run(cmd)
	if err != nil {
		log.Errorf("failed to create initial profile for new system: %v", err)
		return err
	}

	return nil
}

const (
	bootloaderTemplate = `
mount --rbind --mkdir / '%s'
mount --make-rslave '%s'
/run/current-system/bin/switch-to-configuration boot
umount -R '%s' && rmdir '%s'
`
)

func installBootloader(s system.CommandRunner, root string) error {
	bootloaderScript := fmt.Sprintf(bootloaderTemplate, root, root, root, root)
	mtabLocation := filepath.Join(root, "etc", "mtab")

	log := s.Logger()

	err := os.Symlink("/proc/mounts", mtabLocation)
	if err != nil {
		if !errors.Is(err, os.ErrExist) {
			log.Errorf("unable to symlink /proc/mounts to '%v': %v; this is required for bootloader installation", mtabLocation, err)
			return err
		}
	}

	argv := []string{os.Args[0], "enter", "--root", root, "-c", bootloaderScript}
	if log.GetLogLevel() == logger.LogLevelDebug {
		argv = append(argv, "-v")
	} else {
		argv = append(argv, "-s")
	}

	log.CmdArray(argv)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	cmd.SetEnv("NIXOS_INSTALL_BOOTLOADER", "1")
	cmd.SetEnv("NIXOS_CLI_DISABLE_STEPS", "1")

	_, err = s.Run(cmd)
	if err != nil {
		log.Errorf("failed to install bootloader: %v", err)
		return err
	}

	return nil
}

func setRootPassword(s system.CommandRunner, mountpoint string) error {
	log := s.Logger()
	argv := []string{os.Args[0], "enter", "--root", mountpoint, "-c", "/nix/var/nix/profiles/system/sw/bin/passwd"}

	if log.GetLogLevel() == logger.LogLevelDebug {
		argv = append(argv, "-v")
	} else {
		argv = append(argv, "-s")
	}

	s.Logger().CmdArray(argv)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	cmd.SetEnv("NIXOS_CLI_DISABLE_STEPS", "1")

	_, err := s.Run(cmd)
	return err
}

func installMain(cmd *cobra.Command, opts *cmdOpts.InstallOpts) error {
	log := logger.FromContext(cmd.Context())
	s := system.NewLocalSystem(log)

	if !s.IsNixOS() {
		msg := "this command can only be run on NixOS systems"
		log.Error(msg)
		return fmt.Errorf("%v", msg)
	}

	mountpoint, err := filepath.EvalSymlinks(opts.Root)
	if err != nil {
		log.Errorf("failed to resolve root directory: %v", err)
		return err
	}

	if err := validateMountpoint(log, mountpoint); err != nil {
		return err
	}
	tmpDirname, err := os.MkdirTemp(mountpoint, "system")
	if err != nil {
		log.Errorf("failed to create temporary directory: %v", err)
		return err
	}
	defer func() {
		err = os.RemoveAll(tmpDirname)
		if err != nil {
			log.Warnf("unable to remove temporary directory %s, please remove manually", tmpDirname)
		}
	}()

	// Find config location. Do not use the config utils to find the configuration,
	// since the configuration must be specified explicitly. We must avoid
	// the assumptions about `NIX_PATH` containing `nixos-config`, since it
	// refers to the installer's configuration, not the target one to install.
	log.Step("Finding configuration...")

	var nixConfig configuration.Configuration
	if build.Flake() {
		nixConfig = opts.FlakeRef
		log.Debugf("using flake ref %s", opts.FlakeRef)
	} else if opts.File != "" {
		configPath, err := utils.ResolveNixFilename(opts.File)
		if err != nil {
			log.Error(err)
			return err
		}

		nixConfig = &configuration.LegacyConfiguration{
			Includes:        opts.NixOptions.Includes,
			ConfigPath:      configPath,
			Attribute:       opts.Attr,
			UseExplicitPath: true,
		}

		log.Debugf("found configuration at %s", configPath)
		if opts.Attr != "" {
			log.Debugf("using attribute %s", opts.Attr)
		}
	} else {
		var configLocation string

		if nixosCfg, set := os.LookupEnv("NIXOS_CONFIG"); set {
			log.Debug("$NIXOS_CONFIG is set, using automatically")
			configLocation = nixosCfg
		} else {
			configLocation = filepath.Join(mountpoint, "etc", "nixos", "configuration.nix")
		}

		resolvedLocation, err := utils.ResolveNixFilename(configLocation)
		if err != nil {
			return err
		}

		log.Debugf("using configuration at %s", configLocation)

		nixConfig = &configuration.LegacyConfiguration{
			Includes:   opts.NixOptions.Includes,
			ConfigPath: resolvedLocation,
		}
	}
	nixConfig.SetBuilder(s)

	if !opts.NoChannelCopy {
		log.Step("Copying channel...")

		err = copyChannel(cmd, s, mountpoint, opts.Channel, opts.NixOptions)
		if err != nil {
			return err
		}
	}

	envMap := map[string]string{}
	if os.Getenv("TMPDIR") == "" {
		envMap["TMPDIR"] = tmpDirname
	}

	if c, ok := nixConfig.(*configuration.LegacyConfiguration); ok {
		// This value gets appended to the list of includes,
		// and does not replace existing values already provided
		// for -I on the command line.
		if err := cmd.Flags().Set("include", fmt.Sprintf("nixos-config=%s", c.ConfigPath)); err != nil {
			panic("failed to set --include flag for nixos install command for legacy systems")
		}
	}

	var resultLocation string

	if opts.SystemClosure == "" {
		systemBuildOptions := configuration.SystemBuildOptions{
			CmdFlags:  cmd.Flags(),
			NixOpts:   opts.NixOptions,
			Env:       envMap,
			ExtraArgs: []string{"--extra-substituters", defaultExtraSubstituters},
		}

		log.Step("Building system...")

		resultLocation, err = nixConfig.BuildSystem(&configuration.SystemBuild{}, &systemBuildOptions)
		if err != nil {
			log.Errorf("failed to build system: %v", err)
			return err
		}
	} else {
		resultLocation = opts.SystemClosure
	}

	log.Step("Creating initial generation...")

	if err := createInitialGeneration(s, mountpoint, resultLocation); err != nil {
		return err
	}

	// Create /etc/NIXOS file to mark this system as a NixOS system to
	// NixOS tooling such as `switch-to-configuration.pl`.
	log.Step("Creating NixOS indicator")

	etcDirname := filepath.Join(mountpoint, "etc")
	err = os.MkdirAll(etcDirname, 0o755)
	if err != nil {
		log.Errorf("failed to create %v directory: %v", etcDirname, err)
		return err
	}

	etcNixosFilename := filepath.Join(mountpoint, constants.NixOSMarker)
	etcNixos, err := os.Create(etcNixosFilename)
	if err != nil {
		log.Errorf("failed to create %v marker: %v", etcNixosFilename, err)
		return err
	}
	_ = etcNixos.Close()

	if !opts.NoBootloader {
		log.Step("Installing bootloader...")

		if err := installBootloader(s, mountpoint); err != nil {
			return err
		}
	}

	if !opts.NoRootPassword {
		log.Step("Setting root password...")

		manualHint := fmt.Sprintf("you can set the root password manually by executing `nixos enter --root %s` and then running `passwd` in the shell of the new system", mountpoint)

		if !term.IsTerminal(int(os.Stdin.Fd())) {
			log.Warn("stdin is not a terminal; skipping setting root password")
			log.Info(manualHint)
		} else {
			err := setRootPassword(s, mountpoint)
			if err != nil {
				log.Warnf("failed to set root password: %v", err)
				log.Info(manualHint)
			}
		}
	}

	log.Print("Installation successful! You may now reboot.")

	return nil
}
