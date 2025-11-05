package apply

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/nix-community/nixos-cli/internal/activation"
	"github.com/nix-community/nixos-cli/internal/build"
	"github.com/nix-community/nixos-cli/internal/cmd/nixopts"
	cmdOpts "github.com/nix-community/nixos-cli/internal/cmd/opts"
	cmdUtils "github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/configuration"
	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/nix-community/nixos-cli/internal/utils"
	"github.com/spf13/cobra"
)

func ApplyCommand(cfg *settings.Settings) *cobra.Command {
	opts := cmdOpts.ApplyOpts{}

	usage := "apply"
	if build.Flake() {
		usage += " [FLAKE-REF]"
	}

	cmd := cobra.Command{
		Use:   usage,
		Short: "Build/activate a NixOS configuration",
		Long:  "Build and activate a NixOS system from a given configuration.",
		Args: func(cmd *cobra.Command, args []string) error {
			if build.Flake() {
				if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
					return err
				}
				if len(args) > 0 {
					opts.FlakeRef = args[0]
				}
			} else {
				if err := cobra.NoArgs(cmd, args); err != nil {
					return err
				}
			}

			if opts.NoActivate && opts.NoBoot {
				if opts.InstallBootloader {
					return fmt.Errorf("--install-bootloader requires activation, remove --no-activate and/or --no-boot to use this option")
				}

				if opts.OutputPath == "" {
					return fmt.Errorf("if --no-activate and --no-boot are both specified, --output must be specified too")
				}
			}

			if build.Flake() && opts.GenerationTag != "" && !opts.NixOptions.Impure {
				if cfg.Apply.ImplyImpureWithTag {
					if err := cmd.Flags().Set("impure", "true"); err != nil {
						panic("failed to set --impure flag for apply command before exec with explicit generation tag")
					}
				} else {
					return fmt.Errorf("--impure is required when using --tag for flake configurations")
				}
			}

			// Set a special hidden _list value for this
			// flag in order to list available images and
			// exit.
			if cmd.Flags().Changed("image") && opts.BuildImage == "" {
				opts.BuildImage = "_list"
			}

			return nil
		},
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
			return cmdUtils.CommandErrorHandler(applyMain(cmd, &opts))
		},
	}

	cmd.Flags().BoolVarP(&opts.Dry, "dry", "d", false, "Show what would be built or ran")
	cmd.Flags().BoolVar(&opts.InstallBootloader, "install-bootloader", false, "(Re)install the bootloader on the configured device(s)")
	cmd.Flags().BoolVar(&opts.NoActivate, "no-activate", false, "Do not activate the built configuration")
	cmd.Flags().BoolVar(&opts.NoBoot, "no-boot", false, "Do not create boot entry for this generation")
	cmd.Flags().StringVarP(&opts.BuildImage, "image", "i", "", "Build a pre-configured disk-image `variant`")
	cmd.Flags().StringVarP(&opts.OutputPath, "output", "o", "", "Symlink the output to `location`")
	cmd.Flags().StringVarP(&opts.ProfileName, "profile-name", "p", "system", "Store generations using the profile `name`")
	cmd.Flags().StringVarP(&opts.Specialisation, "specialisation", "s", "", "Activate the specialisation with `name`")
	cmd.Flags().StringVarP(&opts.GenerationTag, "tag", "t", "", "Tag this generation with a `description`")
	cmd.Flags().BoolVar(&opts.UseNom, "use-nom", false, "Use 'nix-output-monitor' to build configuration")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", opts.Verbose, "Show verbose logging")
	cmd.Flags().BoolVar(&opts.BuildVM, "vm", false, "Build a NixOS VM script")
	cmd.Flags().BoolVar(&opts.BuildVMWithBootloader, "vm-with-bootloader", false, "Build a NixOS VM script with a bootloader")
	cmd.Flags().BoolVar(&opts.RemoteRoot, "remote-root", false, "Prefix activation commands with an escalation command like sudo")
	cmd.Flags().BoolVarP(&opts.AlwaysConfirm, "yes", "y", false, "Automatically confirm activation")
	cmd.Flags().StringVar(&opts.BuildHost, "build-host", "", "Use specified `user@host:port` to perform build")
	cmd.Flags().StringVar(&opts.TargetHost, "target-host", "", "Deploy to a remote machine at `user@host:port`")

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
	nixopts.AddSubstituteOnDestinationNixOption(&cmd, &opts.NixOptions.SubstituteOnDestination)
	nixopts.AddMaxJobsNixOption(&cmd, &opts.NixOptions.MaxJobs)
	nixopts.AddCoresNixOption(&cmd, &opts.NixOptions.Cores)
	nixopts.AddBuildersNixOption(&cmd, &opts.NixOptions.Builders)
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

	if !build.Flake() {
		cmd.Flags().BoolVar(&opts.UpgradeChannels, "upgrade", false, "Upgrade the root user`s 'nixos' channel")
		cmd.Flags().BoolVar(&opts.UpgradeAllChannels, "upgrade-all", false, "Upgrade all the root user's channels")
	}

	_ = cmd.RegisterFlagCompletionFunc("profile-name", generation.CompleteProfileFlag)
	_ = cmd.RegisterFlagCompletionFunc("specialisation", generation.CompleteSpecialisationFlagFromConfig(opts.FlakeRef, opts.NixOptions.Includes))

	cmd.MarkFlagsMutuallyExclusive("dry", "output")
	cmd.MarkFlagsMutuallyExclusive("output", "build-host")
	cmd.MarkFlagsMutuallyExclusive("output", "target-host")
	cmd.MarkFlagsMutuallyExclusive("vm", "vm-with-bootloader", "image")
	cmd.MarkFlagsMutuallyExclusive("no-activate", "specialisation")

	helpTemplate := cmd.HelpTemplate()
	if build.Flake() {
		helpTemplate += `
Arguments:
  [FLAKE-REF]  Flake ref to build configuration from (default: $NIXOS_CONFIG)
`
	}
	helpTemplate += `
This command also forwards Nix options passed here to all relevant Nix invocations.
Check the man page nixos-cli-apply(1) for more details on what options are available.
`

	cmdUtils.SetHelpFlagText(&cmd)
	cmd.SetHelpTemplate(helpTemplate)

	return &cmd
}

func applyMain(cmd *cobra.Command, opts *cmdOpts.ApplyOpts) error {
	log := logger.FromContext(cmd.Context())
	cfg := settings.FromContext(cmd.Context())
	localSystem := system.NewLocalSystem(log)

	var targetHost system.System

	if opts.TargetHost != "" {
		log.Debugf("connecting to %s", opts.TargetHost)
		host, err := system.NewSSHSystem(opts.TargetHost, log)
		if err != nil {
			log.Errorf("%v", err)
			return err
		}

		defer host.Close()
		targetHost = host
	} else {
		targetHost = localSystem
	}

	if !targetHost.IsNixOS() {
		var err error
		switch targetHost.(type) {
		case *system.SSHSystem:
			err = fmt.Errorf("target host %s is not a NixOS system", opts.TargetHost)
		case *system.LocalSystem:
			err = fmt.Errorf("this system is not a NixOS system")
		}
		log.Error(err)
		return err
	}

	buildType := configuration.SystemBuildTypeSystemActivation
	if opts.BuildVM {
		buildType = configuration.SystemBuildTypeVM
	} else if opts.BuildVMWithBootloader {
		buildType = configuration.SystemBuildTypeVMWithBootloader
	} else if opts.NoActivate && opts.NoBoot {
		buildType = configuration.SystemBuildTypeSystem
	}

	// The local host may need to re-execute as root in order
	// to gain access to activation commands. Do this as early as
	// possible to prevent excessive initialization code from
	// running.
	if os.Geteuid() != 0 && !targetHost.IsRemote() && buildType == configuration.SystemBuildTypeSystemActivation {
		err := utils.ExecAsRoot(cfg.RootCommand)
		if err != nil {
			log.Errorf("failed to re-exec command as root: %v", err)
			return err
		}
	}

	var buildHost system.System

	if opts.BuildHost != "" {
		log.Debugf("connecting to %s", opts.BuildHost)
		host, err := system.NewSSHSystem(opts.BuildHost, log)
		if err != nil {
			log.Errorf("%v", err)
			return err
		}

		defer host.Close()
		buildHost = host
	} else {
		buildHost = localSystem
	}

	log.Step("Looking for configuration...")

	var nixConfig configuration.Configuration
	if opts.FlakeRef != "" {
		nixConfig = configuration.FlakeRefFromString(opts.FlakeRef)
		if err := nixConfig.(*configuration.FlakeRef).InferSystemFromHostnameIfNeeded(); err != nil {
			log.Errorf("failed to infer hostname: %v", err)
			return err
		}
	} else {
		c, err := configuration.FindConfiguration(log, cfg, opts.NixOptions.Includes)
		if err != nil {
			log.Errorf("failed to find configuration: %v", err)
			return err
		}
		nixConfig = c
	}

	nixConfig.SetBuilder(buildHost)

	var configDirname string
	switch c := nixConfig.(type) {
	case *configuration.FlakeRef:
		configDirname = c.URI
	case *configuration.LegacyConfiguration:
		configDirname = c.ConfigDirname
	}

	configIsDirectory := true
	originalCwd, err := os.Getwd()
	if err != nil {
		log.Errorf("failed to get current directory: %v", err)
		return err
	}
	if configDirname != "" {
		// Change to the configuration directory, if it exists:
		// this will likely fail for remote configurations or
		// configurations accessed through the registry, which
		// should be a rare occurrence, but valid, so ignore any
		// errors in that case.
		err := os.Chdir(configDirname)
		if err != nil {
			configIsDirectory = false
		}
	}

	if !build.Flake() && (opts.UpgradeChannels || opts.UpgradeAllChannels) {
		log.Step("Upgrading channels...")

		if err := upgradeChannels(localSystem, &upgradeChannelsOptions{
			UpgradeAll: opts.UpgradeAllChannels,
		}); err != nil {
			log.Warnf("failed to update channels: %v", err)
			log.Warnf("continuing with existing channels")
		}
	}

	if buildType.IsVM() {
		log.Step("Building VM...")
	} else {
		log.Step("Building configuration...")
	}

	useNom := cfg.Apply.UseNom || opts.UseNom
	nomFound := buildHost.HasCommand("nom")
	if opts.UseNom && !nomFound {
		err := fmt.Errorf("--use-nom was specified, but `nom` is not executable")
		log.Error(err)
		return err
	} else if cfg.Apply.UseNom && !nomFound {
		log.Warn("apply.use_nom is specified in config, but `nom` is not executable")
		log.Warn("falling back to `nix` command for building")
		useNom = false
	}

	generationTag := opts.GenerationTag
	if generationTag == "" {
		if tagVar := os.Getenv("NIXOS_GENERATION_TAG"); tagVar != "" {
			log.Debugf("using explicitly set NIXOS_GENERATION_TAG variable for generation tag")
			generationTag = tagVar
		}
	}

	if generationTag == "" && cfg.Apply.UseGitCommitMsg {
		if !configIsDirectory {
			log.Warn("configuration is not a directory")
		} else {
			commitMsg, err := getLatestGitCommitMessage(configDirname, cfg.Apply.IgnoreDirtyTree)
			if err == errDirtyGitTree {
				log.Warn("git tree is dirty")
			} else if err != nil {
				log.Warnf("failed to get latest git commit message: %v", err)
			} else {
				generationTag = commitMsg
			}
		}
	}

	generationTag = strings.TrimSpace(generationTag)

	if generationTag != "" {
		// Make sure --impure is added to the Nix options if
		// an implicit commit message is used.
		if err := cmd.Flags().Set("impure", "true"); err != nil {
			panic("failed to set --impure flag for apply command before exec with implicit generation tag with git message")
		}
	}

	// Dry activation requires a real build, so --dry-run shouldn't be set
	// if --activate or --boot is set
	dryBuild := opts.Dry && buildType == configuration.SystemBuildTypeSystem

	outputPath := opts.OutputPath
	if outputPath != "" && !filepath.IsAbs(outputPath) {
		outputPath = filepath.Join(originalCwd, outputPath)
	}

	buildOptions := &configuration.SystemBuildOptions{
		ResultLocation: outputPath,
		DryBuild:       dryBuild,
		UseNom:         useNom,
		GenerationTag:  generationTag,

		CmdFlags: cmd.Flags(),
		NixOpts:  &opts.NixOptions,
	}

	resultLocation, err := nixConfig.BuildSystem(buildType, buildOptions)
	if err != nil {
		log.Errorf("failed to build configuration: %v", err)
		return err
	}

	if !dryBuild {
		copyFlags := nixopts.NixOptionsToArgsListByCategory(cmd.Flags(), opts.NixOptions, "copy")
		err := system.CopyClosures(buildHost, targetHost, []string{resultLocation}, copyFlags...)
		if err != nil {
			log.Errorf("failed to copy system closure to target host: %v", err)
			return err
		}
	}

	if buildType.IsVM() && !dryBuild {
		matches, err := filepath.Glob(fmt.Sprintf("%v/bin/run-*-vm", resultLocation))
		if err != nil || len(matches) == 0 {
			log.Warnf("failed to find VM binary; look in %v for the script to run the VM", resultLocation)
		} else {
			log.Infof("done; the virtual machine can be started by running `%v`", matches[0])
		}
		return nil
	}

	if buildType == configuration.SystemBuildTypeSystem {
		if dryBuild {
			log.Debugf("this is a dry build, no activation will be performed")
		}
		return nil
	}

	log.Step("Comparing changes...")

	err = generation.RunDiffCommand(targetHost, constants.CurrentSystem, resultLocation, &generation.DiffCommandOptions{
		UseNvd: cfg.UseNvd,
	})
	if err != nil {
		log.Errorf("failed to run diff command: %v", err)
	}

	if !opts.AlwaysConfirm {
		log.Printf("\n")
		confirm, err := cmdUtils.ConfirmationInput("Activate this configuration?")
		if err != nil {
			log.Errorf("failed to get confirmation: %v", err)
			return err
		}
		if !confirm {
			msg := "confirmation was not given, skipping activation"
			log.Warn(msg)
			return fmt.Errorf("%v", msg)
		}
	}

	if t, ok := targetHost.(*system.SSHSystem); ok && opts.RemoteRoot {
		err = t.EnsureRemoteRootPassword(cfg.RootCommand)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	specialisation := opts.Specialisation
	if specialisation == "" {
		defaultSpecialisation, err := activation.FindDefaultSpecialisationFromConfig(targetHost, resultLocation)
		if err != nil {
			log.Warnf("unable to find default specialisation from config: %v", err)
		} else {
			specialisation = defaultSpecialisation
		}
	}

	if !activation.VerifySpecialisationExists(targetHost, resultLocation, specialisation) {
		log.Warnf("specialisation '%v' does not exist", specialisation)
		log.Warn("using base configuration without specialisations")
		specialisation = ""
	}

	previousGenNumber, err := activation.GetCurrentGenerationNumber(targetHost, opts.ProfileName)
	if err != nil {
		log.Errorf("%v", err)
		return err
	}

	// Do not create a generation for dry runs or for
	// testing generations using the --no-boot option.
	createGeneration := !opts.Dry && !opts.NoBoot

	if createGeneration {
		log.Step("Setting system profile...")

		if err := activation.AddNewNixProfile(
			targetHost,
			opts.ProfileName,
			resultLocation,
			&activation.AddNewNixProfileOptions{
				RootCommand:    cfg.RootCommand,
				UseRootCommand: opts.RemoteRoot && targetHost.IsRemote(),
			},
		); err != nil {
			log.Errorf("failed to set system profile: %v", err)
			return err
		}
	}

	// In case switch-to-configuration fails, rollback the profile.
	// This is to prevent accidental deletion of all working
	// generations in case the switch-to-configuration script
	// fails, since the active profile will not be rolled back
	// automatically.
	rollbackProfile := false
	if createGeneration {
		defer func(rollback *bool) {
			if !*rollback {
				return
			}

			if !cfg.AutoRollback {
				log.Warnf("automatic rollback is disabled, the currently active profile may have unresolved problems")
				log.Warnf("you are on your own!")
				return
			}

			log.Step("Rolling back system profile...")

			if err := activation.SetNixProfileGeneration(
				targetHost,
				opts.ProfileName,
				previousGenNumber, &activation.SetNixProfileGenerationOptions{
					RootCommand:    cfg.RootCommand,
					UseRootCommand: opts.RemoteRoot && targetHost.IsRemote(),
				},
			); err != nil {
				log.Errorf("failed to rollback system profile: %v", err)
				log.Info("make sure to rollback the system manually before deleting anything!")
			}
		}(&rollbackProfile)
	}

	log.Step("Activating...")

	var stcAction activation.SwitchToConfigurationAction
	if opts.Dry && !opts.NoActivate {
		stcAction = activation.SwitchToConfigurationActionDryActivate
	} else if !opts.NoActivate && !opts.NoBoot {
		stcAction = activation.SwitchToConfigurationActionSwitch
	} else if opts.NoActivate && !opts.NoBoot {
		stcAction = activation.SwitchToConfigurationActionBoot
	} else if !opts.NoActivate && opts.NoBoot {
		stcAction = activation.SwitchToConfigurationActionTest
	} else {
		panic("unknown switch to configuration action to take, this is a bug")
	}

	err = activation.SwitchToConfiguration(targetHost, resultLocation, stcAction, &activation.SwitchToConfigurationOptions{
		InstallBootloader: opts.InstallBootloader,
		Specialisation:    specialisation,
		UseRootCommand:    opts.RemoteRoot && targetHost.IsRemote(),
		RootCommand:       cfg.RootCommand,
	})
	if err != nil {
		rollbackProfile = true
		log.Errorf("failed to switch to configuration: %v", err)
		return err
	}

	return nil
}

const channelDirectory = constants.NixProfileDirectory + "/per-user/root/channels"

type upgradeChannelsOptions struct {
	UpgradeAll bool
}

func upgradeChannels(s system.CommandRunner, opts *upgradeChannelsOptions) error {
	argv := []string{"nix-channel", "--update"}

	if !opts.UpgradeAll {
		// Always upgrade the `nixos` channel, as well as any channels that
		// have the ".update-on-nixos-rebuild" marker file in them.
		argv = append(argv, "nixos")

		entries, err := os.ReadDir(channelDirectory)
		if err != nil {
			return err
		}

		for _, entry := range entries {
			if entry.IsDir() {
				if _, err := os.Stat(filepath.Join(channelDirectory, entry.Name(), ".update-on-nixos-rebuild")); err == nil {
					argv = append(argv, entry.Name())
				}
			}
		}
	}

	s.Logger().CmdArray(argv)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	_, err := s.Run(cmd)
	return err
}

var errDirtyGitTree = fmt.Errorf("git tree is dirty")

func getLatestGitCommitMessage(pathToRepo string, ignoreDirty bool) (string, error) {
	repo, err := git.PlainOpen(pathToRepo)
	if err != nil {
		return "", err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return "", err
	}

	status, err := wt.Status()
	if err != nil {
		return "", err
	}

	if !status.IsClean() && !ignoreDirty {
		return "", errDirtyGitTree
	}

	head, err := repo.Head()
	if err != nil {
		return "", err
	}

	commit, err := repo.CommitObject(head.Hash())
	if err != nil {
		return "", err
	}

	return commit.Message, nil
}
