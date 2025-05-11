package install

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	buildOpts "github.com/water-sucks/nixos/internal/build"
	"github.com/water-sucks/nixos/internal/cmd/nixopts"
	"github.com/water-sucks/nixos/internal/cmd/opts"
	"github.com/water-sucks/nixos/internal/cmd/utils"
	"github.com/water-sucks/nixos/internal/configuration"
	"github.com/water-sucks/nixos/internal/constants"
	"github.com/water-sucks/nixos/internal/logger"
	"github.com/water-sucks/nixos/internal/system"
	"golang.org/x/term"
)

func InstallCommand() *cobra.Command {
	opts := cmdOpts.InstallOpts{}

	usage := "install"
	if buildOpts.Flake == "true" {
		usage += " {FLAKE-URI}#{SYSTEM-NAME}"
	}

	cmd := cobra.Command{
		Use:   usage,
		Short: "Install a NixOS system",
		Long:  "Install a NixOS system from a given configuration.",
		Args: func(cmd *cobra.Command, args []string) error {
			if buildOpts.Flake == "true" {
				if err := cobra.ExactArgs(1)(cmd, args); err != nil {
					return err
				}

				ref := configuration.FlakeRefFromString(args[0])
				if ref.System == "" {
					return fmt.Errorf("missing required argument {SYSTEM-NAME}")
				}
				opts.FlakeRef = ref
			} else {
				if err := cobra.NoArgs(cmd, args); err != nil {
					return err
				}
			}
			return nil
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if len(opts.Root) > 0 && !filepath.IsAbs(opts.Root) {
				return fmt.Errorf("--root must be an absolute path")
			}
			if len(opts.SystemClosure) > 0 && !filepath.IsAbs(opts.SystemClosure) {
				return fmt.Errorf("--system must be an absolute path")
			}
			return nil
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

	if buildOpts.Flake == "true" {
		nixopts.AddRecreateLockFileNixOption(&cmd, &opts.NixOptions.RecreateLockFile)
		nixopts.AddNoUpdateLockFileNixOption(&cmd, &opts.NixOptions.NoUpdateLockFile)
		nixopts.AddNoWriteLockFileNixOption(&cmd, &opts.NixOptions.NoWriteLockFile)
		nixopts.AddNoUseRegistriesNixOption(&cmd, &opts.NixOptions.NoUseRegistries)
		nixopts.AddCommitLockFileNixOption(&cmd, &opts.NixOptions.CommitLockFile)
		nixopts.AddUpdateInputNixOption(&cmd, &opts.NixOptions.UpdateInputs)
		nixopts.AddOverrideInputNixOption(&cmd, &opts.NixOptions.OverrideInputs)
	}

	cmd.MarkFlagsMutuallyExclusive("channel", "no-channel-copy")

	helpTemplate := cmd.HelpTemplate()
	if buildOpts.Flake == "true" {
		helpTemplate += `
Arguments:
  [FLAKE-URI]    Flake URI that contains NixOS system to build
  [SYSTEM-NAME]  Name of NixOS system attribute to build
`
	}
	helpTemplate += `
This command also forwards Nix options passed here to all relevant Nix invocations.
Check the Nix manual page for more details on what options are available.
`

	cmdUtils.SetHelpFlagText(&cmd)
	cmd.SetHelpTemplate(helpTemplate)

	return &cmd
}

func validateMountpoint(log *logger.Logger, mountpoint string) error {
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

func copyChannel(cobraCmd *cobra.Command, s system.CommandRunner, log *logger.Logger, mountpoint string, channelDirectory string, buildOptions any, verbose bool) error {
	mountpointChannelDir := filepath.Join(mountpoint, constants.NixChannelDirectory)

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
	if verbose {
		log.CmdArray(argv)
	}

	_, err := s.Run(cmd)
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

func createInitialGeneration(s system.CommandRunner, mountpoint string, closure string, verbose bool) error {
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

	if verbose {
		log.CmdArray(argv)
	}

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

func installBootloader(s system.CommandRunner, root string, verbose bool) error {
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
	if verbose {
		argv = append(argv, "-v")
	} else {
		argv = append(argv, "-s")
	}

	if verbose {
		log.CmdArray(argv)
	}

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

func setRootPassword(s system.CommandRunner, mountpoint string, verbose bool) error {
	argv := []string{os.Args[0], "enter", "--root", mountpoint, "-c", "/nix/var/nix/profiles/system/sw/bin/passwd"}

	if verbose {
		argv = append(argv, "-v")
	} else {
		argv = append(argv, "-s")
	}

	if verbose {
		s.Logger().CmdArray(argv)
	}

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

	if opts.Verbose {
		log.Step("Finding configuration...")
	}

	var nixConfig configuration.Configuration
	if buildOpts.Flake == "true" {
		nixConfig = opts.FlakeRef
	} else {
		var configLocation string

		if nixosCfg, set := os.LookupEnv("NIXOS_CONFIG"); set {
			if opts.Verbose {
				log.Info("$NIXOS_CONFIG is set, using automatically")
			}
			configLocation = nixosCfg
		} else {
			configLocation = filepath.Join(mountpoint, "etc", "nixos", "configuration.nix")
		}

		if _, err := os.Stat(configLocation); err != nil {
			log.Errorf("failed to stat %s: %v", configLocation, err)
			return err
		}

		nixConfig = &configuration.LegacyConfiguration{
			Includes:      opts.NixOptions.Includes,
			ConfigDirname: configLocation,
		}
	}
	nixConfig.SetBuilder(s)

	log.Step("Copying channel...")

	err = copyChannel(cmd, s, log, mountpoint, opts.Channel, opts.NixOptions, opts.Verbose)
	if err != nil {
		return err
	}

	envMap := map[string]string{}
	if os.Getenv("TMPDIR") == "" {
		envMap["TMPDIR"] = tmpDirname
	}

	if c, ok := nixConfig.(*configuration.LegacyConfiguration); ok {
		opts.NixOptions.Includes = append(opts.NixOptions.Includes, fmt.Sprintf("nixos-config=%s", c.ConfigDirname))
	}
	systemBuildOptions := configuration.SystemBuildOptions{
		Verbose:   opts.Verbose,
		CmdFlags:  cmd.Flags(),
		NixOpts:   opts.NixOptions,
		Env:       envMap,
		ExtraArgs: []string{"--extra-substituters", defaultExtraSubstituters},
	}

	log.Step("Building system...")

	resultLocation, err := nixConfig.BuildSystem(configuration.SystemBuildTypeSystem, &systemBuildOptions)
	if err != nil {
		log.Errorf("failed to build system: %v", err)
		return err
	}

	log.Step("Creating initial generation...")

	if err := createInitialGeneration(s, mountpoint, resultLocation, opts.Verbose); err != nil {
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

	log.Step("Installing bootloader...")

	if err := installBootloader(s, mountpoint, opts.Verbose); err != nil {
		return err
	}

	log.Step("Setting root password...")

	if !opts.NoRootPassword {
		manualHint := "you can set the root password manually by executing `nixos enter --root {s}` and then running `passwd` in the shell of them new system"

		if !term.IsTerminal(int(os.Stdin.Fd())) {
			log.Warn("stdin is not a terminal; skipping setting root password")
			log.Info(manualHint)
		} else {
			err := setRootPassword(s, mountpoint, opts.Verbose)
			if err != nil {
				log.Warnf("failed to set root password: %v", err)
				log.Info(manualHint)
			}
		}
	}

	log.Print("Installation successful! You may now reboot.")

	return nil
}
