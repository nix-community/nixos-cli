package apply

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

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
	"github.com/nix-community/nixos-cli/internal/nix"
	"github.com/nix-community/nixos-cli/internal/settings"
	sshUtils "github.com/nix-community/nixos-cli/internal/ssh"
	"github.com/nix-community/nixos-cli/internal/system"
	systemdUtils "github.com/nix-community/nixos-cli/internal/systemd"
	"github.com/nix-community/nixos-cli/internal/utils"
	"github.com/spf13/cobra"
)

func ApplyCommand(cfg *settings.Settings) *cobra.Command {
	opts := cmdOpts.ApplyOpts{}

	var usage string
	if build.Flake() {
		usage = "apply [FLAKE-REF]"
	} else {
		usage = "apply [FILE] [ATTR]"
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

			if opts.NoActivate && opts.NoBoot {
				if opts.InstallBootloader {
					return fmt.Errorf("--install-bootloader requires activation, remove --no-activate and/or --no-boot to use this option")
				}

				if opts.StorePath != "" {
					return fmt.Errorf("--store-path skips building, remove --no-activate and/or --no-boot to use this option")
				}
			}

			if build.Flake() && opts.GenerationTag != "" && !bool(opts.NixOptions.Impure) {
				if cfg.Apply.ImplyImpureWithTag {
					if err := cmd.Flags().Set("impure", "true"); err != nil {
						panic("failed to set --impure flag for apply command before exec with explicit generation tag")
					}
				} else {
					return fmt.Errorf("--impure is required when using --tag for flake configurations")
				}
			}

			if opts.StorePath != "" {
				if build.Flake() && opts.FlakeRef != "" || !build.Flake() && opts.File != "" {
					var nixConfigArg string
					if build.Flake() {
						nixConfigArg = "[FLAKE-REF]"
					} else {
						nixConfigArg = "[FILE]"
					}
					return fmt.Errorf("--store-path was specified, but %v was also provided; use one or the other", nixConfigArg)
				}

				if opts.BuildHost == "" {
					storePath, err := utils.ResolveDirectory(opts.StorePath)
					if err != nil {
						return fmt.Errorf("failed to resolve store path: %v", err)
					}
					opts.StorePath = storePath
				}
			}

			// Make sure rollback-timeout is a valid systemd.time(7) string
			if timeout, err := systemdUtils.DurationFromTimeSpan(opts.RollbackTimeout); err != nil {
				return fmt.Errorf("invalid value for --rollback-timeout: %v", err.Error())
			} else if timeout < 1*time.Second {
				return errors.New("--rollback-timeout must be at least 1 second")
			}

			// Set a special hidden _list value for this
			// flag in order to list available images and
			// exit.
			if cmd.Flags().Changed("image") && opts.BuildImage == "" {
				opts.BuildImage = "_list"
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
	cmd.Flags().BoolVar(&opts.EvalOnly, "eval-only", false, "Only evaluate the configuration without building or activating")
	cmd.Flags().BoolVar(&opts.UseNom, "use-nom", false, "Use 'nix-output-monitor' to build configuration")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", opts.Verbose, "Show verbose logging")
	cmd.Flags().BoolVar(&opts.BuildVM, "vm", false, "Build a NixOS VM script")
	cmd.Flags().BoolVar(&opts.BuildVMWithBootloader, "vm-with-bootloader", false, "Build a NixOS VM script with a bootloader")
	cmd.Flags().BoolVar(&opts.LocalRoot, "local-root", false, "Prefix local activation and channel upgrade commands with an escalation command like sudo")
	cmd.Flags().BoolVar(&opts.RemoteRoot, "remote-root", false, "Prefix remote activation commands with an escalation command like sudo")
	cmd.Flags().BoolVarP(&opts.AlwaysConfirm, "yes", "y", false, "Automatically confirm activation")
	cmd.Flags().StringVar(&opts.BuildHost, "build-host", "", "Use specified `user@host:port` to perform build")
	cmd.Flags().StringVar(&opts.TargetHost, "target-host", "", "Deploy to a remote machine at `user@host:port`")
	cmd.Flags().StringVar(&opts.StorePath, "store-path", "", "Use a pre-built NixOS system store `path` instead of building")
	cmd.Flags().StringVar(&opts.RollbackTimeout, "rollback-timeout", "30s", "Time `period` to wait for acknowledgement signal before automatic rollback")
	cmd.Flags().BoolVar(&opts.NoRollback, "no-rollback", false, "Do not attempt rollback after a switch failure")

	opts.NixOptions.Quiet.Bind(&cmd)
	opts.NixOptions.PrintBuildLogs.Bind(&cmd)
	opts.NixOptions.NoBuildOutput.Bind(&cmd)
	opts.NixOptions.ShowTrace.Bind(&cmd)
	opts.NixOptions.KeepGoing.Bind(&cmd)
	opts.NixOptions.KeepFailed.Bind(&cmd)
	opts.NixOptions.Fallback.Bind(&cmd)
	opts.NixOptions.Refresh.Bind(&cmd)
	opts.NixOptions.Repair.Bind(&cmd)
	opts.NixOptions.Impure.Bind(&cmd)
	opts.NixOptions.Offline.Bind(&cmd)
	opts.NixOptions.NoNet.Bind(&cmd)
	opts.NixOptions.SubstituteOnDestination.Bind(&cmd)
	opts.NixOptions.MaxJobs.Bind(&cmd)
	opts.NixOptions.Cores.Bind(&cmd)
	opts.NixOptions.Builders.Bind(&cmd)
	opts.NixOptions.LogFormat.Bind(&cmd)
	opts.NixOptions.Option.Bind(&cmd)
	opts.NixOptions.Include.Bind(&cmd)

	if build.Flake() {
		opts.NixOptions.RecreateLockFile.Bind(&cmd)
		opts.NixOptions.NoUpdateLockFile.Bind(&cmd)
		opts.NixOptions.NoWriteLockFile.Bind(&cmd)
		opts.NixOptions.NoUseRegistries.Bind(&cmd)
		opts.NixOptions.CommitLockFile.Bind(&cmd)
		opts.NixOptions.UpdateInput.Bind(&cmd)
		opts.NixOptions.OverrideInput.Bind(&cmd)
	}

	if !build.Flake() {
		cmd.Flags().BoolVar(&opts.UpgradeChannels, "upgrade", false, "Upgrade the root user`s 'nixos' channel")
		cmd.Flags().BoolVar(&opts.UpgradeAllChannels, "upgrade-all", false, "Upgrade all the root user's channels")
	}

	_ = cmd.RegisterFlagCompletionFunc("profile-name", generation.CompleteProfileFlag)
	_ = cmd.RegisterFlagCompletionFunc("specialisation", generation.CompleteSpecialisationFlagFromConfig(opts.FlakeRef, opts.NixOptions.Include))
	_ = cmd.RegisterFlagCompletionFunc("store-path", cmdUtils.DirCompletions)

	cmd.MarkFlagsMutuallyExclusive("dry", "output")
	cmd.MarkFlagsMutuallyExclusive("vm", "vm-with-bootloader", "image", "store-path")
	cmd.MarkFlagsMutuallyExclusive("no-activate", "specialisation")
	cmd.MarkFlagsMutuallyExclusive("eval-only", "store-path")
	cmd.MarkFlagsMutuallyExclusive("no-rollback", "rollback-timeout")

	helpTemplate := cmd.HelpTemplate()
	if build.Flake() {
		helpTemplate += `
Arguments:
  [FLAKE-REF]  Flake ref to build configuration from (default: $NIXOS_CONFIG)
`
	} else {
		helpTemplate += `
Arguments:
  [FILE]  File to build configuration from
  [ATTR]  Attribute inside of [FILE] pointing to configuration

  Both arguments are optional. If [FILE] is not specified, then $NIXOS_CONFIG or the 'nixos-config'
  entry in $NIX_PATH is used. If [ATTR] is not specified, then the top-level attribute of [FILE]
  is used.
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

type errRequiresLocalRoot struct {
	Action string
}

func (e errRequiresLocalRoot) Error() string {
	return fmt.Sprintf("%v requires passing '--local-root' or enabling 'apply.reexec_as_root'", e.Action)
}

func applyMain(cmd *cobra.Command, opts *cmdOpts.ApplyOpts) error {
	log := logger.FromContext(cmd.Context())
	cfg := settings.FromContext(cmd.Context())
	localSystem := system.NewLocalSystem(log)

	stopCtx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	sshAgent, err := sshUtils.NewAgentManager(log)
	if err != nil {
		log.Warnf("failed to start/connect to SSH agent: %v", err)
	}
	defer func() {
		if sshAgent != nil {
			_ = sshAgent.Stop()
		}
	}()

	var targetHost system.System

	if opts.TargetHost != "" {
		log.Debugf("connecting to %s", opts.TargetHost)

		var sshCfg *system.SSHConfig
		sshCfg, err = system.NewSSHConfig(stopCtx, opts.TargetHost, log, system.SSHConfigOptions{
			AgentManager:    sshAgent,
			KnownHostsFiles: cfg.SSH.KnownHostsFiles,
			PrivateKeyCmd:   cfg.SSH.PrivateKeyCmd,
		})
		if err != nil {
			log.Errorf("%v", err)
			return err
		}

		var host *system.SSHSystem
		host, err = system.NewSSHSystem(sshCfg, log)
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
		switch targetHost.(type) {
		case *system.SSHSystem:
			err = fmt.Errorf("target host %s is not a NixOS system", opts.TargetHost)
		case *system.LocalSystem:
			err = fmt.Errorf("this system is not a NixOS system")
		}
		log.Error(err)
		return err
	}

	var buildType configuration.BuildType
	if opts.BuildVM || opts.BuildVMWithBootloader {
		buildType = &configuration.VMBuild{WithBootloader: opts.BuildVMWithBootloader}
	} else if opts.BuildImage != "" {
		buildType = &configuration.ImageBuild{Variant: opts.BuildImage}
	} else {
		buildType = &configuration.SystemBuild{Activate: !opts.NoActivate || !opts.NoBoot}
	}

	// Dry activation requires a real build, so --dry-run shouldn't be set
	// if running activation scripts.
	dryBuild := opts.Dry
	if v, ok := buildType.(*configuration.SystemBuild); ok && v.Activate {
		dryBuild = false
	}

	// The local host may need to re-execute as root in order
	// to gain access to activation or channel upgrade commands.
	// Do this as early as possible to prevent excessive initialization
	// code from running.
	if os.Geteuid() != 0 && cfg.Apply.ReexecRoot && !targetHost.IsRemote() {
		// Only re-execute if running activation on local system for the
		// system build type or upgrading channels.
		if v, ok := buildType.(*configuration.SystemBuild); ok && v.Activate ||
			!build.Flake() && (opts.UpgradeChannels || opts.UpgradeAllChannels) {
			if err = utils.ExecAsRoot(cfg.RootCommand); err != nil {
				log.Errorf("failed to re-exec command as root: %v", err)
				return err
			}
		}
	}
	effectiveRoot := os.Geteuid() == 0

	var buildHost system.System

	if opts.BuildHost != "" {
		log.Debugf("connecting to %s", opts.BuildHost)

		var sshCfg *system.SSHConfig
		sshCfg, err = system.NewSSHConfig(stopCtx, opts.BuildHost, log, system.SSHConfigOptions{
			AgentManager:    sshAgent,
			KnownHostsFiles: cfg.SSH.KnownHostsFiles,
			PrivateKeyCmd:   cfg.SSH.PrivateKeyCmd,
		})
		if err != nil {
			log.Errorf("%v", err)
			return err
		}

		var host *system.SSHSystem
		host, err = system.NewSSHSystem(sshCfg, log)
		if err != nil {
			log.Errorf("%v", err)
			return err
		}
		defer host.Close()

		buildHost = host
	} else {
		buildHost = localSystem
	}

	var nixConfig configuration.Configuration
	var resultLocation string

	if opts.StorePath == "" {
		log.Step("Looking for configuration...")

		if opts.FlakeRef != "" {
			nixConfig = configuration.FlakeRefFromString(opts.FlakeRef)
			if err = nixConfig.(*configuration.FlakeRef).InferSystemFromHostnameIfNeeded(); err != nil {
				log.Errorf("failed to infer hostname: %v", err)
				return err
			}
		} else if opts.File != "" {
			var configPath string
			configPath, err = utils.ResolveNixFilename(opts.File)
			if err != nil {
				log.Error(err)
				return err
			}

			nixConfig = &configuration.LegacyConfiguration{
				Includes:        opts.NixOptions.Include,
				ConfigPath:      configPath,
				Attribute:       opts.Attr,
				UseExplicitPath: true,
			}

			log.Debugf("found configuration at %s", configPath)
			if opts.Attr != "" {
				log.Debugf("using attribute '%s'", opts.Attr)
			}
		} else {
			var c configuration.Configuration
			c, err = configuration.FindConfiguration(log, cfg, opts.NixOptions.Include)
			if err != nil {
				log.Errorf("failed to find configuration: %v", err)
				return err
			}
			nixConfig = c
		}

		nixConfig.SetBuilder(buildHost)

		if opts.EvalOnly {
			switch buildType.(type) {
			case *configuration.SystemBuild:
				log.Step("Evaluating configuration...")
			case *configuration.ImageBuild:
				log.Step("Evaluating image...")
			case *configuration.VMBuild:
				log.Step("Evaluating VM...")
			}

			evalOptions := &configuration.SystemEvalOptions{
				NixOpts: &opts.NixOptions,
			}

			var drvPath string
			drvPath, err = nixConfig.EvalSystem(localSystem, buildType, evalOptions)
			if err != nil {
				log.Errorf("failed to evaluate configuration: %v", err)
				return err
			}

			log.Infof("the evaluated derivation is %v", drvPath)
			return nil
		}

		if !build.Flake() && (opts.UpgradeChannels || opts.UpgradeAllChannels) {
			log.Step("Upgrading channels...")

			if !effectiveRoot && !opts.LocalRoot {
				err := errRequiresLocalRoot{Action: "upgrading channels"}
				log.Error(err)
				return err
			}

			if err = upgradeChannels(localSystem, &upgradeChannelsOptions{
				UpgradeAll:     opts.UpgradeAllChannels,
				RootCommand:    cfg.RootCommand,
				UseRootCommand: !effectiveRoot && opts.LocalRoot,
			}); err != nil {
				log.Warnf("failed to update channels: %v", err)
				log.Warnf("continuing with existing channels")
			}
		}

		// Confirm if the requested image exists, or list available images
		// if no parameter is specified/is empty.
		if imgBuild, ok := buildType.(*configuration.ImageBuild); ok {
			var images []string
			images, err = getAvailableImageAttrs(localSystem, nixConfig, &opts.NixOptions)
			if err != nil {
				log.Errorf("failed to get available images: %v", err)
				return err
			}

			if imgBuild.Variant == "_list" {
				for _, image := range images {
					fmt.Println(image)
				}
				return nil
			}

			if slices.Index(images, imgBuild.Variant) < 0 {
				err = fmt.Errorf("image type '%s' is not available", imgBuild.Variant)
				log.Error(err)
				log.Info("pass an empty string to `--image` to get a list of available images")
				return err
			}
		}

		switch buildType.(type) {
		case *configuration.SystemBuild:
			log.Step("Building configuration...")
		case *configuration.ImageBuild:
			log.Step("Building image...")
		case *configuration.VMBuild:
			log.Step("Building VM...")
		}

		useNom := cfg.Apply.UseNom || opts.UseNom
		nomFound := buildHost.HasCommand("nom")
		if opts.UseNom && !nomFound {
			err = fmt.Errorf("--use-nom was specified, but `nom` is not executable")
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
			var configDirname string
			switch c := nixConfig.(type) {
			case *configuration.FlakeRef:
				configDirname = c.URI
			case *configuration.LegacyConfiguration:
				configDirname = c.Dirname()
			}
			configDirname, err = utils.ResolveDirectory(configDirname)

			if err != nil {
				log.Warnf("failed to resolve configuration path: %v", err)
			} else {
				var commitMsg string
				commitMsg, err = getLatestGitCommitMessage(configDirname, cfg.Apply.IgnoreDirtyTree)
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
			if err = cmd.Flags().Set("impure", "true"); err != nil {
				panic("failed to set --impure flag for apply command before exec with implicit generation tag with git message")
			}
		}

		buildOptions := &configuration.SystemBuildOptions{
			ResultLocation: opts.OutputPath,
			DryBuild:       dryBuild,
			UseNom:         useNom,
			GenerationTag:  generationTag,

			CmdFlags: cmd.Flags(),
			NixOpts:  &opts.NixOptions,
		}

		resultLocation, err = nixConfig.BuildSystem(buildType, buildOptions)
		if err != nil {
			log.Errorf("failed to build configuration: %v", err)
			return err
		}
	} else {
		resultLocation = opts.StorePath
	}

	if !dryBuild {
		copyFlags := opts.NixOptions.ArgsForCommand(nixopts.CmdCopyClosure)
		err = system.CopyClosures(buildHost, targetHost, []string{resultLocation}, copyFlags...)
		if err != nil {
			log.Errorf("failed to copy system closure to target host: %v", err)
			return err
		}
	}

	switch v := buildType.(type) {
	case *configuration.ImageBuild:
		if !dryBuild {
			var imagePath string
			imagePath, err = getImageName(localSystem, nixConfig, v.Variant, &opts.NixOptions)
			if err != nil {
				log.Infof("finished building image in %s", resultLocation)
			} else {
				location := filepath.Join(resultLocation, imagePath)
				log.Infof("done; the built image is located at %s", location)
			}
		}

		return nil
	case *configuration.VMBuild:
		if !dryBuild {
			var matches []string
			matches, err = filepath.Glob(fmt.Sprintf("%v/bin/run-*-vm", resultLocation))
			if err != nil || len(matches) == 0 {
				log.Warnf("failed to find VM binary; look in %v for the script to run the VM", resultLocation)
			} else {
				log.Infof("done; the virtual machine can be started by running `%v`", matches[0])
			}
		}

		return nil
	case *configuration.SystemBuild:
		if dryBuild {
			log.Debugf("this is a dry build, no activation will be performed")
		} else if opts.NoActivate && opts.NoBoot {
			log.Infof("the built configuration is %v", resultLocation)
		}

		if !v.Activate {
			return nil
		}
	}

	log.Step("Comparing changes...")

	err = generation.RunDiffCommand(targetHost, constants.CurrentSystem, resultLocation, &generation.DiffCommandOptions{
		UseNvd: cfg.UseNvd,
	})
	if err != nil {
		log.Errorf("failed to run diff command: %v", err)
	}

	if !opts.AlwaysConfirm && !cfg.Confirmation.Always {
		log.Printf("\n")

		var confirm bool
		confirm, err = cmdUtils.ConfirmationInput("Activate this configuration?", cmdUtils.ConfirmationPromptOptions{
			InvalidBehavior: cfg.Confirmation.Invalid,
			EmptyBehavior:   cfg.Confirmation.Empty,
		})
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
		err = t.EnsureRemoteRootPassword(stopCtx, cfg.RootCommand)
		if err != nil {
			log.Error(err)
			return err
		}
	}

	specialisation := opts.Specialisation
	if specialisation == "" {
		var defaultSpecialisation string
		defaultSpecialisation, err = activation.FindDefaultSpecialisationFromConfig(targetHost, resultLocation)
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

	activationMissingRoot := !effectiveRoot && !opts.LocalRoot && !targetHost.IsRemote()
	activationUseRoot := !effectiveRoot && opts.LocalRoot && !targetHost.IsRemote() || opts.RemoteRoot && targetHost.IsRemote()

	// Do not create a generation for dry runs or for
	// testing generations using the --no-boot option.
	createGeneration := !opts.Dry && !opts.NoBoot

	// New generations may not always be created, in the case of
	// running `apply` on the same exact configuration.
	// Track this here, and do not perform a system profile
	// rollback if one hasn't been created.
	newGenerationCreated := false

	if createGeneration {
		log.Step("Setting system profile...")

		activeProfileLink := generation.GetProfileDirectoryFromName(opts.ProfileName)

		var prevLink string
		prevLink, err = targetHost.FS().ReadLink(activeProfileLink)
		if err != nil {
			log.Errorf("%v", err)
			return err
		}

		if activationMissingRoot {
			err := errRequiresLocalRoot{Action: "setting a system profile locally"}
			log.Error(err)
			return err
		}

		if err = activation.AddNewNixProfile(
			targetHost,
			opts.ProfileName,
			resultLocation,
			&activation.AddNewNixProfileOptions{
				RootCommand:    cfg.RootCommand,
				UseRootCommand: activationUseRoot,
			},
		); err != nil {
			log.Errorf("failed to set system profile: %v", err)
			return err
		}

		var afterLink string
		afterLink, err = targetHost.FS().ReadLink(activeProfileLink)
		if err != nil {
			log.Errorf("%v", err)
			return err
		}

		if prevLink != afterLink {
			newGenerationCreated = true
		}
	}

	rollbackLocalProfile := func() {
		if !cfg.AutoRollback {
			log.Warnf("automatic rollback is disabled, the currently active profile may have unresolved problems")
			log.Warnf("you are on your own!")
			return
		}

		log.Step("Rolling back system profile...")

		if rbErr := activation.SetNixProfileGeneration(
			targetHost,
			opts.ProfileName,
			previousGenNumber, &activation.SetNixProfileGenerationOptions{
				RootCommand:    cfg.RootCommand,
				UseRootCommand: activationUseRoot,
			},
		); rbErr != nil {
			log.Errorf("failed to rollback system profile: %v", rbErr)
			log.Info("make sure to rollback the system manually before deleting anything!")
		}
	}

	log.Step("Activating...")

	if activationMissingRoot {
		err := errRequiresLocalRoot{Action: "running switch-to-configuration locally"}
		log.Error(err)
		return err
	}

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

	useActivationSupervisor := shouldUseActivationSupervisor(cfg, targetHost, stcAction) && !opts.NoRollback

	if useActivationSupervisor {
		ackTimeout, _ := systemdUtils.DurationFromTimeSpan(opts.RollbackTimeout)
		ackTimeout = ackTimeout / time.Second

		// Let the supervisor handle the rollback if it exists.
		err = activation.RunActivationSupervisor(targetHost, resultLocation, stcAction, &activation.RunActivationSupervisorOptions{
			ProfileName:       opts.ProfileName,
			InstallBootloader: opts.InstallBootloader,
			Specialisation:    specialisation,
			UseRootCommand:    activationUseRoot,
			RootCommand:       cfg.RootCommand,
			AckTimeout:        ackTimeout,

			// TODO: figure out previous specialisation, set it here
			// so that we can rollback directly to a given specialisation
			// by setting it in these options.

			// Only rollback the system profile if a new generation was created.
			RollbackProfileOnFailure: newGenerationCreated,
		})
		if err != nil {
			log.Errorf("%v", err)
			return err
		}
	} else {
		// Otherwise, just use the switch-to-configuration script directly
		// and handle profile rollback ourselves.
		err = activation.SwitchToConfiguration(targetHost, resultLocation, stcAction, &activation.SwitchToConfigurationOptions{
			InstallBootloader: opts.InstallBootloader,
			Specialisation:    specialisation,
			UseRootCommand:    activationUseRoot,
			RootCommand:       cfg.RootCommand,
		})
		if err != nil {
			log.Errorf("failed to switch to configuration: %v", err)
			if !opts.NoRollback && newGenerationCreated {
				rollbackLocalProfile()
			}
			return err
		}
	}

	return nil
}

const channelDirectory = constants.NixProfileDirectory + "/per-user/root/channels"

type upgradeChannelsOptions struct {
	UpgradeAll     bool
	RootCommand    string
	UseRootCommand bool
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
				if _, err = os.Stat(filepath.Join(channelDirectory, entry.Name(), ".update-on-nixos-rebuild")); err == nil {
					argv = append(argv, entry.Name())
				}
			}
		}
	}

	s.Logger().CmdArray(argv)

	cmd := system.NewCommand(argv[0], argv[1:]...)
	if opts.UseRootCommand {
		cmd.AsRoot(opts.RootCommand)
	}
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

func getAvailableImageAttrs(
	s system.System,
	cfg configuration.Configuration,
	nixOpts *cmdOpts.ApplyNixOpts,
) ([]string, error) {
	var argv []string
	var attr string

	switch v := cfg.(type) {
	case *configuration.FlakeRef:
		evalArgs := nixOpts.ArgsForCommand(nixopts.CmdEval)

		attr = v.BuildAttr("images")
		argv = []string{"nix", "eval", "--json", attr, "--apply", "builtins.attrNames"}
		argv = append(argv, evalArgs...)
	case *configuration.LegacyConfiguration:
		instantiateArgs := nixOpts.ArgsForCommand(nixopts.CmdInstantiate)

		expr := "with import <nixpkgs/nixos> {}; builtins.attrNames config.system.build.images"
		argv = []string{"nix-instantiate", "--eval", "--strict", "--json", "--expr", expr}
		argv = append(argv, instantiateArgs...)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := system.NewCommand(argv[0], argv[1:]...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	_, err := s.Run(cmd)
	if err != nil {
		return nil, &configuration.AttributeEvaluationError{
			Attribute:        attr,
			EvaluationOutput: stderr.String(),
		}
	}

	var imageAttrs []string
	err = json.NewDecoder(&stdout).Decode(&imageAttrs)
	if err != nil {
		return nil, err
	}

	return imageAttrs, nil
}

func getImageName(
	s system.System,
	cfg configuration.Configuration,
	imgName string,
	nixOpts *cmdOpts.ApplyNixOpts,
) (string, error) {
	var argv []string
	var attr string

	imgName = nix.MakeAttrName(imgName)
	switch v := cfg.(type) {
	case *configuration.FlakeRef:
		evalArgs := nixOpts.ArgsForCommand(nixopts.CmdEval)

		attr = v.BuildAttr("images", imgName, "passthru", "filePath")
		argv = []string{"nix", "eval", "--raw", attr}
		argv = append(argv, evalArgs...)
	case *configuration.LegacyConfiguration:
		instantiateArgs := nixOpts.ArgsForCommand(nixopts.CmdInstantiate)

		expr := fmt.Sprintf("with import <nixpkgs/nixos> {}; config.system.build.images.%s.passthru.filePath", imgName)
		argv = []string{"nix-instantiate", "--eval", "--strict", "--raw", "--expr", expr}

		argv = append(argv, instantiateArgs...)
	}

	var stdout bytes.Buffer
	var stderr bytes.Buffer

	cmd := system.NewCommand(argv[0], argv[1:]...)
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	s.Logger().CmdArray(argv)

	_, err := s.Run(cmd)
	if err != nil {
		return "", &configuration.AttributeEvaluationError{
			Attribute:        attr,
			EvaluationOutput: stderr.String(),
		}
	}

	return strings.TrimSpace(stdout.String()), nil
}

func shouldUseActivationSupervisor(cfg *settings.Settings, host system.System, action activation.SwitchToConfigurationAction) bool {
	if !cfg.AutoRollback || !host.IsRemote() {
		return false
	}

	isValidAction := action == activation.SwitchToConfigurationActionBoot ||
		action == activation.SwitchToConfigurationActionSwitch ||
		action == activation.SwitchToConfigurationActionTest

	return isValidAction
}
