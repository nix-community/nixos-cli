package switch_cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/nix-community/nixos-cli/internal/activation"
	"github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/nix-community/nixos-cli/internal/utils"
)

func GenerationSwitchCommand(genOpts *cmdOpts.GenerationOpts) *cobra.Command {
	opts := cmdOpts.GenerationSwitchOpts{}

	cmd := cobra.Command{
		Use:   "switch [flags] {GEN}",
		Short: "Activate an existing generation",
		Long:  "Activate an arbitrary existing NixOS generation",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.ExactArgs(1)(cmd, args); err != nil {
				return err
			}

			arg := args[0]
			gen, err := strconv.ParseInt(arg, 10, 32)
			if err != nil {
				return fmt.Errorf("{GEN} must be integer value, got '%v'", arg)
			}

			opts.Generation = uint(gen)

			return nil
		},
		ValidArgsFunction: generation.CompleteGenerationNumber(&genOpts.ProfileName, 1),
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
			return cmdUtils.CommandErrorHandler(generationSwitchMain(cmd, genOpts, &opts))
		},
	}

	cmd.Flags().BoolVarP(&opts.Dry, "dry", "d", false, "Show what would be activated, but do not activate")
	cmd.Flags().StringVarP(&opts.Specialisation, "specialisation", "s", "", "Activate the specialisation with `name`")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Show verbose logging")
	cmd.Flags().BoolVarP(&opts.AlwaysConfirm, "yes", "y", false, "Automatically confirm activation")

	_ = cmd.RegisterFlagCompletionFunc("specialisation", completeSpecialisationFlag(genOpts.ProfileName))

	cmdUtils.SetHelpFlagText(&cmd)
	cmd.SetHelpTemplate(cmd.HelpTemplate() + `
Arguments:
    [GEN]       Generation number
`)

	return &cmd
}

func completeSpecialisationFlag(profileName string) cobra.CompletionFunc {
	profileDirectory := constants.NixProfileDirectory
	if profileName != "system" {
		profileDirectory = constants.NixSystemProfileDirectory
	}

	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		// Cobra does not parse out the generation number, because it has not
		// validated them yet.
		// Let's pull it out ourselves based on the first provided
		// command-line positional argument
		if len(args) < 1 {
			return []string{}, cobra.ShellCompDirectiveNoFileComp
		}
		arg := args[0]

		genNumber, err := strconv.ParseInt(arg, 10, 32)
		if err != nil {
			return []string{}, cobra.ShellCompDirectiveNoFileComp
		}

		generationLink := filepath.Join(profileDirectory, fmt.Sprintf("%v-%v-link", profileName, genNumber))

		return generation.CompleteSpecialisationFlag(generationLink)(cmd, args, toComplete)
	}
}

func generationSwitchMain(cmd *cobra.Command, genOpts *cmdOpts.GenerationOpts, opts *cmdOpts.GenerationSwitchOpts) error {
	log := logger.FromContext(cmd.Context())
	cfg := settings.FromContext(cmd.Context())
	s := system.NewLocalSystem(log)

	if os.Geteuid() != 0 {
		err := utils.ExecAsRoot(cfg.RootCommand)
		if err != nil {
			log.Errorf("failed to re-exec command as root: %v", err)
			return err
		}
	}

	profileDirectory := constants.NixProfileDirectory
	if genOpts.ProfileName != "system" {
		profileDirectory = constants.NixSystemProfileDirectory
	}
	generationLink := filepath.Join(profileDirectory, fmt.Sprintf("%v-%v-link", genOpts.ProfileName, opts.Generation))

	// Check if generation exists. There are rare cases in which a Nix profile can
	// point to a nonexistent store path, such as in the case that someone manually
	// deletes stuff, but this shouldn't really happen much, if at all.
	if _, err := os.Stat(generationLink); err != nil {
		if os.IsNotExist(err) {
			msg := fmt.Sprintf("generation %v not found", opts.Generation)
			log.Error(msg)
			return fmt.Errorf("%v", msg)
		}

		log.Errorf("failed to access generation link: %v", err)
		return err
	}

	log.Step("Comparing changes...")

	err := generation.RunDiffCommand(s, constants.CurrentSystem, generationLink, &generation.DiffCommandOptions{
		UseNvd: cfg.UseNvd,
	})
	if err != nil {
		log.Errorf("failed to run diff command: %v", err)
	}

	if !opts.AlwaysConfirm {
		log.Printf("\n")
		confirm, err := cmdUtils.ConfirmationInput("Activate this generation?")
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
		defaultSpecialisation, err := activation.FindDefaultSpecialisationFromConfig(generationLink)
		if err != nil {
			log.Warnf("unable to find default specialisation from config: %v", err)
		} else {
			specialisation = defaultSpecialisation
		}
	}

	if !activation.VerifySpecialisationExists(generationLink, specialisation) {
		log.Warnf("specialisation '%v' does not exist", specialisation)
		log.Warn("using base configuration without specialisations")
		specialisation = ""
	}

	previousGenNumber, err := activation.GetCurrentGenerationNumber(genOpts.ProfileName)
	if err != nil {
		log.Errorf("%v", err)
		return err
	}

	if !opts.Dry {
		log.Step("Setting system profile...")

		if err := activation.SetNixProfileGeneration(s, genOpts.ProfileName, uint64(opts.Generation)); err != nil {
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

			if !cfg.AutoRollback {
				log.Warnf("automatic rollback is disabled, the currently active profile may have unresolved problems")
				log.Warnf("you are on your own!")
				return
			}

			log.Step("Rolling back system profile...")
			if err := activation.SetNixProfileGeneration(s, genOpts.ProfileName, previousGenNumber); err != nil {
				log.Errorf("failed to rollback system profile: %v", err)
				log.Info("make sure to rollback the system manually before deleting anything!")
			}
		}(&rollbackProfile)
	}

	log.Step("Activating...")

	var stcAction activation.SwitchToConfigurationAction = activation.SwitchToConfigurationActionSwitch
	if opts.Dry {
		stcAction = activation.SwitchToConfigurationActionDryActivate
	}

	err = activation.SwitchToConfiguration(s, generationLink, stcAction, &activation.SwitchToConfigurationOptions{
		Specialisation: specialisation,
	})
	if err != nil {
		rollbackProfile = true
		log.Errorf("failed to switch to configuration: %v", err)
		return err
	}

	return nil
}
