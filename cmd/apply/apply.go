package apply

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/spf13/cobra"
	"github.com/water-sucks/nixos/internal/activation"
	buildOpts "github.com/water-sucks/nixos/internal/build"
	"github.com/water-sucks/nixos/internal/cmd/nixopts"
	cmdTypes "github.com/water-sucks/nixos/internal/cmd/types"
	cmdUtils "github.com/water-sucks/nixos/internal/cmd/utils"
	"github.com/water-sucks/nixos/internal/configuration"
	"github.com/water-sucks/nixos/internal/constants"
	"github.com/water-sucks/nixos/internal/generation"
	"github.com/water-sucks/nixos/internal/logger"
	"github.com/water-sucks/nixos/internal/settings"
	"github.com/water-sucks/nixos/internal/system"
	"github.com/water-sucks/nixos/internal/utils"
)

func ApplyCommand(cfg *settings.Settings) *cobra.Command {
	opts := cmdTypes.ApplyOpts{}

	usage := "apply"
	if buildOpts.Flake == "true" {
		usage += " [FLAKE-REF]"
	}

	cmd := cobra.Command{
		Use:   usage,
		Short: "Build/activate a NixOS configuration",
		Long:  "Build and activate a NixOS system from a given configuration.",
		Args: func(cmd *cobra.Command, args []string) error {
			if buildOpts.Flake == "true" {
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
			return nil
		},
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if opts.NoActivate && opts.NoBoot && opts.InstallBootloader {
				return fmt.Errorf("--install-bootloader requires activation, remove --no-activate and/or --no-boot to use this option")
			}
			if buildOpts.Flake == "true" && opts.GenerationTag != "" && !opts.NixOptions.Impure {
				if cfg.Apply.ImplyImpureWithTag {
					opts.NixOptions.Impure = true
				} else {
					return fmt.Errorf("--impure is required when using --tag for flake configurations")
				}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdUtils.CommandErrorHandler(applyMain(cmd, &opts))
		},
	}

	cmd.Flags().BoolVarP(&opts.Dry, "dry", "d", false, "Show what would be built or ran")
	cmd.Flags().BoolVar(&opts.InstallBootloader, "install-bootloader", false, "(Re)install the bootloader on the configured device(s)")
	cmd.Flags().BoolVar(&opts.NoActivate, "no-activate", false, "Do not activate the built configuration")
	cmd.Flags().BoolVar(&opts.NoBoot, "no-boot", false, "Do not create boot entry for this generation")
	cmd.Flags().StringVarP(&opts.OutputPath, "output", "o", "", "Symlink the output to `location`")
	cmd.Flags().StringVarP(&opts.ProfileName, "profile-name", "p", "system", "Store generations using the profile `name`")
	cmd.Flags().StringVarP(&opts.Specialisation, "specialisation", "s", "", "Activate the specialisation with `name`")
	cmd.Flags().StringVarP(&opts.GenerationTag, "tag", "t", "", "Tag this generation with a `description`")
	cmd.Flags().BoolVar(&opts.UseNom, "use-nom", false, "Use 'nix-output-monitor' to build configuration")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", opts.Verbose, "Show verbose logging")
	cmd.Flags().BoolVar(&opts.BuildVM, "vm", false, "Build a NixOS VM script")
	cmd.Flags().BoolVar(&opts.BuildVMWithBootloader, "vm-with-bootloader", false, "Build a NixOS VM script with a bootloader")
	cmd.Flags().BoolVarP(&opts.AlwaysConfirm, "yes", "y", false, "Automatically confirm activation")

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
	nixopts.AddBuildersNixOption(&cmd, &opts.NixOptions.Builders)
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

	if buildOpts.Flake == "false" {
		cmd.Flags().BoolVar(&opts.UpgradeChannels, "upgrade", false, "Upgrade the root user`s 'nixos' channel")
		cmd.Flags().BoolVar(&opts.UpgradeAllChannels, "upgrade-all", false, "Upgrade all the root user's channels")
	}

	err := cmd.RegisterFlagCompletionFunc("profile-name", generation.CompleteProfileFlag)
	if err != nil {
		panic("failed to register flag completion function: " + err.Error())
	}

	cmd.MarkFlagsMutuallyExclusive("dry", "output")
	cmd.MarkFlagsMutuallyExclusive("vm", "vm-with-bootloader")
	cmd.MarkFlagsMutuallyExclusive("no-activate", "specialisation")

	helpTemplate := cmd.HelpTemplate()
	if buildOpts.Flake == "true" {
		helpTemplate += `
Arguments:
  [FLAKE-REF]  Flake ref to build configuration from (default: $NIXOS_CONFIG)
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

func applyMain(cmd *cobra.Command, opts *cmdTypes.ApplyOpts) error {
	log := logger.FromContext(cmd.Context())
	cfg := settings.FromContext(cmd.Context())
	s := system.NewLocalSystem(log)

	if !s.IsNixOS() {
		msg := "this command only is only supported on NixOS systems"
		log.Errorf(msg)
		return fmt.Errorf("%v", msg)
	}

	buildType := configuration.SystemBuildTypeSystemActivation
	if opts.BuildVM {
		buildType = configuration.SystemBuildTypeVM
	} else if opts.BuildVMWithBootloader {
		buildType = configuration.SystemBuildTypeVMWithBootloader
	} else if opts.NoActivate && opts.NoBoot {
		buildType = configuration.SystemBuildTypeSystem
	}

	if os.Geteuid() != 0 {
		err := utils.ExecAsRoot(cfg.RootCommand)
		if err != nil {
			log.Errorf("failed to re-exec command as root: %v", err)
			return err
		}
	}

	if opts.Verbose {
		log.Step("Looking for configuration...")
	}

	var nixConfig configuration.Configuration
	if opts.FlakeRef != "" {
		nixConfig = configuration.FlakeRefFromString(opts.FlakeRef)
	} else {
		c, err := configuration.FindConfiguration(log, cfg, opts.NixOptions.Includes, opts.Verbose)
		if err != nil {
			log.Errorf("failed to find configuration: %v", err)
			return err
		}
		nixConfig = c
	}

	nixConfig.SetBuilder(s)

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

	if buildOpts.Flake != "true" && (opts.UpgradeChannels || opts.UpgradeAllChannels) {
		log.Step("Upgrading channels...")

		if err := upgradeChannels(s, &upgradeChannelsOptions{
			UpgradeAll: opts.UpgradeAllChannels,
			Verbose:    opts.Verbose,
		}); err != nil {
			log.Warnf("failed to update channels: %v", err)
			log.Warnf("continuing with existing channels", err)
		}
	}

	if buildType.IsVM() {
		log.Step("Building VM...")
	} else {
		log.Step("Building configuration...")
	}

	useNom := cfg.Apply.UseNom || opts.UseNom
	nomPath, _ := exec.LookPath("nom")
	nomFound := nomPath != ""
	if opts.UseNom && !nomFound {
		log.Error("--use-nom was specified, but `nom` is not executable")
	} else if cfg.Apply.UseNom && !nomFound {
		log.Warn("apply.use_nom is specified in config, but `nom` is not executable")
		log.Warn("falling back to `nix` command for building")
		useNom = false
	}

	generationTag := opts.GenerationTag
	if generationTag == "" && cfg.Apply.UseGitCommitMsg {
		if !configIsDirectory {
			log.Warn("configuration is not a directory")
		} else {
			commitMsg, err := getLatestGitCommitMessage(configDirname)
			if err == dirtyGitTreeError {
				log.Warnf("failed to get latest git commit message: %v", err)
			} else if err != nil {
				log.Warn("git tree is dirty")
			} else {
				generationTag = commitMsg
			}
		}

		generationTag = strings.TrimSpace(generationTag)

		if generationTag == "" {
			log.Warn("ignoring apply.use_git_commit_msg setting")
		} else {
			// Make sure --impure is added to the nix options if
			// an implicit commit message is used.
			opts.NixOptions.Impure = true
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
		Verbose:        opts.Verbose,

		CmdFlags: cmd.Flags(),
		NixOpts:  &opts.NixOptions,
	}

	resultLocation, err := nixConfig.BuildSystem(buildType, buildOptions)
	if err != nil {
		log.Errorf("failed to build configuration: %v", err)
		return err
	}

	if buildType.IsVM() && !dryBuild {
		matches, err := filepath.Glob(fmt.Sprintf("%v/bin/run-*-vm", resultLocation))
		if err != nil || len(matches) == 0 {
			msg := fmt.Sprintf("Failed to find VM binary; look in %v for the script to run the VM.", resultLocation)
			log.Errorf(msg)
			return fmt.Errorf("%v", msg)
		}
		log.Printf("Done. The virtual machine can be started by running `%v`.\n", matches[0])
		return nil
	}

	if buildType == configuration.SystemBuildTypeSystem && dryBuild {
		if opts.Verbose {
			log.Infof("this is a dry build, no activation will be performed")
		}
		return nil
	}

	log.Step("Comparing changes...")

	err = generation.RunDiffCommand(log, s, constants.CurrentSystem, resultLocation, &generation.DiffCommandOptions{
		UseNvd:  cfg.UseNvd,
		Verbose: opts.Verbose,
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

	specialisation := opts.Specialisation
	if specialisation == "" {
		defaultSpecialisation, err := activation.FindDefaultSpecialisationFromConfig(resultLocation)
		if err != nil {
			log.Warnf("unable to find default specialisation from config: %v", err)
		} else {
			specialisation = defaultSpecialisation
		}
	}

	if !activation.VerifySpecialisationExists(resultLocation, specialisation) {
		log.Warnf("specialisation '%v' does not exist", specialisation)
		log.Warn("using base configuration without specialisations")
		specialisation = ""
	}

	previousGenNumber, err := activation.GetCurrentGenerationNumber(opts.ProfileName)
	if err != nil {
		log.Errorf("%v", err)
		return err
	}

	if !opts.Dry {
		if opts.Verbose {
			log.Step("Setting system profile...")
		}

		if err := activation.AddNewNixProfile(s, opts.ProfileName, resultLocation, opts.Verbose); err != nil {
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
	if !opts.Dry {
		defer func(rollback *bool) {
			if !*rollback {
				return
			}

			log.Step("Rolling back system profile...")
			if err := activation.SetNixProfileGeneration(s, "system", previousGenNumber, opts.Verbose); err != nil {
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
	} else if opts.NoActivate && opts.NoBoot {
		stcAction = activation.SwitchToConfigurationActionTest
	} else {
		panic("unknown switch to configuration action to take, this is a bug")
	}

	err = activation.SwitchToConfiguration(s, resultLocation, stcAction, &activation.SwitchToConfigurationOptions{
		InstallBootloader: opts.InstallBootloader,
		Verbose:           opts.Verbose,
		Specialisation:    specialisation,
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
	Verbose    bool
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

	if opts.Verbose {
		s.Logger().CmdArray(argv)
	}

	cmd := system.NewCommand(argv[0], argv[1:]...)
	_, err := s.Run(cmd)
	return err
}

var dirtyGitTreeError = fmt.Errorf("git tree is dirty")

func getLatestGitCommitMessage(pathToRepo string) (string, error) {
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

	if !status.IsClean() {
		return "", dirtyGitTreeError
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
