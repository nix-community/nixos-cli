package rollback

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"

	"github.com/spf13/cobra"

	"github.com/nix-community/nixos-cli/cmd/generation/shared"
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

func GenerationRollbackCommand(genOpts *cmdOpts.GenerationOpts) *cobra.Command {
	opts := cmdOpts.GenerationRollbackOpts{}

	cmd := cobra.Command{
		Use:   "rollback [flags]",
		Short: "Activate the previous generation",
		Long:  "Rollback to the previous NixOS generation.",
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
			return cmdUtils.CommandErrorHandler(generationRollbackMain(cmd, genOpts, &opts))
		},
	}

	cmd.Flags().BoolVarP(&opts.Dry, "dry", "d", false, "Show what would be activated, but do not activate")
	cmd.Flags().StringVarP(&opts.Specialisation, "specialisation", "s", "", "Activate the specialisation with `name`")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Show verbose logging")
	cmd.Flags().BoolVarP(&opts.AlwaysConfirm, "yes", "y", false, "Automatically confirm activation")

	_ = cmd.RegisterFlagCompletionFunc("specialisation", completeSpecialisationFlag(genOpts.ProfileName))

	cmdUtils.SetHelpFlagText(&cmd)

	return &cmd
}

func generationRollbackMain(cmd *cobra.Command, genOpts *cmdOpts.GenerationOpts, opts *cmdOpts.GenerationRollbackOpts) error {
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

	// While it is possible to use the `rollback` command, we still need
	// to find the previous generation number ourselves in order to run
	// `nvd` or `nix store diff-closures` properly.
	previousGen, err := findPreviousGeneration(log, genOpts.ProfileName)
	if err != nil {
		return err
	}

	profileDirectory := constants.NixProfileDirectory
	if genOpts.ProfileName != "system" {
		profileDirectory = constants.NixSystemProfileDirectory
	}
	generationLink := filepath.Join(profileDirectory, fmt.Sprintf("%v-%v-link", genOpts.ProfileName, previousGen.Number))

	log.Step("Comparing changes...")

	err = generation.RunDiffCommand(s, constants.CurrentSystem, generationLink, &generation.DiffCommandOptions{
		UseNvd: cfg.UseNvd,
	})
	if err != nil {
		log.Errorf("failed to run diff command: %v", err)
	}

	if !opts.AlwaysConfirm && !cfg.Confirmation.Always {
		log.Printf("\n")

		var confirm bool
		confirm, err = cmdUtils.ConfirmationInput("Activate the previous generation?", cmdUtils.ConfirmationPromptOptions{
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

	specialisation := opts.Specialisation
	if specialisation == "" {
		var defaultSpecialisation string
		defaultSpecialisation, err = activation.FindDefaultSpecialisationFromConfig(s, generationLink)
		if err != nil {
			log.Warnf("unable to find default specialisation from config: %v", err)
		} else {
			specialisation = defaultSpecialisation
		}
	}

	if !activation.VerifySpecialisationExists(s, generationLink, specialisation) {
		log.Warnf("specialisation '%v' does not exist", specialisation)
		log.Warn("using base configuration without specialisations")
		specialisation = ""
	}

	previousGenNumber, err := activation.GetCurrentGenerationNumber(s, genOpts.ProfileName)
	if err != nil {
		log.Errorf("%v", err)
		return err
	}

	if !opts.Dry {
		log.Step("Setting system profile...")

		if err = activation.SetNixProfileGeneration(
			s,
			genOpts.ProfileName,
			uint64(previousGen.Number),
			&activation.SetNixProfileGenerationOptions{},
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
			if err = activation.SetNixProfileGeneration(
				s,
				genOpts.ProfileName,
				previousGenNumber,
				&activation.SetNixProfileGenerationOptions{},
			); err != nil {
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

func findPreviousGeneration(log logger.Logger, profileName string) (*generation.Generation, error) {
	generations, err := genUtils.LoadGenerations(log, profileName, false)
	if err != nil {
		return nil, err
	}

	currentGenIdx := slices.IndexFunc(generations, func(g generation.Generation) bool {
		return g.IsCurrent
	})
	if currentGenIdx == -1 {
		panic("current generation not found, this is a bug")
	}
	currentGen := generations[currentGenIdx]

	if currentGenIdx == 0 {
		msg := fmt.Sprintf("no generation older than the current one (%v) exists", currentGen.Number)
		log.Error(msg)
		return nil, fmt.Errorf("%v", msg)
	}

	return &generations[currentGenIdx-1], nil
}

func completeSpecialisationFlag(profileName string) cobra.CompletionFunc {
	profileDirectory := constants.NixProfileDirectory
	if profileName != "system" {
		profileDirectory = constants.NixSystemProfileDirectory
	}

	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		log, _ := cmdUtils.PrepareCompletionResources()

		previousGen, err := findPreviousGeneration(log, profileName)
		if err != nil {
			return []string{}, cobra.ShellCompDirectiveNoFileComp
		}

		generationLink := filepath.Join(profileDirectory, fmt.Sprintf("%v-%v-link", profileName, previousGen.Number))

		return generation.CompleteSpecialisationFlag(generationLink)(cmd, args, toComplete)
	}
}
