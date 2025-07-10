package delete

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/olekukonko/tablewriter"
	"github.com/spf13/cobra"

	"github.com/nix-community/nixos-cli/cmd/generation/shared"
	"github.com/nix-community/nixos-cli/internal/build"
	"github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/nix-community/nixos-cli/internal/system"
	timeUtils "github.com/nix-community/nixos-cli/internal/time"
	"github.com/nix-community/nixos-cli/internal/utils"
)

func GenerationDeleteCommand(genOpts *cmdOpts.GenerationOpts) *cobra.Command {
	opts := cmdOpts.GenerationDeleteOpts{}

	cmd := cobra.Command{
		Use:   "delete [flags] [GEN...]",
		Short: "Delete generations from this system",
		Long:  "Delete NixOS generations from this system.",
		Args: func(cmd *cobra.Command, args []string) error {
			for _, v := range args {
				value, err := strconv.ParseInt(v, 10, 32)
				if err != nil {
					return fmt.Errorf("[GEN] must be integer value, got '%v'", v)
				}
				opts.Remove = append(opts.Remove, uint(value))
			}
			if cmd.Flags().Changed("older-than") {
				// Make sure older-than is a valid systemd.time(7) string
				if _, err := timeUtils.DurationFromTimeSpan(opts.OlderThan); err != nil {
					return fmt.Errorf("invalid value for --older-than: %v", err.Error())
				}
			}

			for _, remove := range opts.Remove {
				for _, keep := range opts.Keep {
					if remove == keep {
						return fmt.Errorf("cannot remove and keep the same generation %v", remove)
					}
				}
			}

			log := logger.FromContext(cmd.Context())

			if opts.All {
				if opts.LowerBound != 0 {
					log.Warn("--all was specified, ignoring --from")
				}
				if opts.OlderThan != "" {
					log.Warn("--all was specified, ignoring --older-than")
				}
				if opts.UpperBound != 0 {
					log.Warn("--all was specified, ignoring --to")
				}
				if len(opts.Remove) != 0 {
					log.Warn("--all was specified, ignoring positional arguments")
				}
			}

			if !opts.All && opts.LowerBound == 0 && opts.UpperBound == 0 && len(opts.Remove) == 0 && opts.OlderThan == "" && len(opts.Keep) == 0 && opts.MinimumToKeep == 0 {
				return fmt.Errorf("no generations or deletion parameters were given")
			}

			return nil
		},
		ValidArgsFunction: generation.CompleteGenerationNumber(&genOpts.ProfileName, 0),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdUtils.CommandErrorHandler(generationDeleteMain(cmd, genOpts, &opts))
		},
	}

	cmd.Flags().BoolVarP(&opts.All, "all", "a", false, "Delete all generations except the current one")
	cmd.Flags().Uint64VarP(&opts.LowerBound, "from", "f", 0, "Delete all generations after `gen`, inclusive")
	cmd.Flags().Uint64VarP(&opts.UpperBound, "to", "t", 0, "Delete all generations until `gen`, inclusive")
	cmd.Flags().Uint64VarP(&opts.MinimumToKeep, "min", "m", 0, "Keep a minimum of `num` generations")
	cmd.Flags().StringVarP(&opts.OlderThan, "older-than", "o", "", "Delete all generations older than `period`")
	cmd.Flags().UintSliceVarP(&opts.Keep, "keep", "k", nil, "Always keep this `gen`, can be specified many times")
	cmd.Flags().BoolVarP(&opts.Verbose, "verbose", "v", false, "Show verbose logging")
	cmd.Flags().BoolVarP(&opts.AlwaysConfirm, "yes", "y", false, "Automatically confirm generation deletion")

	_ = cmd.RegisterFlagCompletionFunc("from", generation.CompleteGenerationNumberFlag(&genOpts.ProfileName))
	_ = cmd.RegisterFlagCompletionFunc("to", generation.CompleteGenerationNumberFlag(&genOpts.ProfileName))
	_ = cmd.RegisterFlagCompletionFunc("keep", generation.CompleteGenerationNumberFlag(&genOpts.ProfileName))

	cmdUtils.SetHelpFlagText(&cmd)
	cmd.SetHelpTemplate(cmd.HelpTemplate() + `
Arguments:
    [GEN]       Generation number

These options and arguments can be combined ad-hoc as constraints.

The 'period' parameter in --older-than is a systemd.time(7) span
(i.e. "30d 2h 1m"). Check the manual page for more information.
`)

	return &cmd
}

func generationDeleteMain(cmd *cobra.Command, genOpts *cmdOpts.GenerationOpts, opts *cmdOpts.GenerationDeleteOpts) error {
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

	generations, err := genUtils.LoadGenerations(log, genOpts.ProfileName, false)
	if err != nil {
		return err
	}

	gensToDelete, err := resolveGenerationsToDelete(generations, opts)
	if err != nil {
		log.Errorf("%v", err)

		switch err.(type) {
		case GenerationResolveMinError:
			log.Info("keeping all generations")
		case GenerationResolveNoneFoundError:
			log.Info("there is nothing to do; exiting")
		}
		return err
	}

	remainingGenCount := len(generations) - len(gensToDelete)

	log.Print("The following generations will be deleted:")
	log.Print()
	displayDeleteSummary(gensToDelete)
	log.Printf("\nThere will be %v generations remaining on this machine.", remainingGenCount)
	log.Print()

	if !opts.AlwaysConfirm {
		confirm, err := cmdUtils.ConfirmationInput("Proceed?")
		if err != nil {
			log.Errorf("failed to get confirmation: %v", err)
			return err
		}
		if !confirm {
			log.Info("confirmation was not given, not proceeding")
			return nil
		}
	}

	log.Step("Deleting generations...")

	profileDirectory := generation.GetProfileDirectoryFromName(genOpts.ProfileName)
	if err := deleteGenerations(s, profileDirectory, gensToDelete, opts.Verbose); err != nil {
		log.Errorf("failed to delete generations: %v", err)
		return err
	}

	log.Step("Regenerating boot menu entries...")

	if err := regenerateBootMenu(s, opts.Verbose); err != nil {
		log.Errorf("failed to regenerate boot menu entries: %v", err)
		return err
	}

	log.Step("Collecting garbage...")

	if err := collectGarbage(s, opts.Verbose); err != nil {
		log.Errorf("failed to collect garbage: %v", err)
		return err
	}

	log.Print("Success!")

	return nil
}

func displayDeleteSummary(generations []generation.Generation) {
	data := make([][]string, len(generations))

	for i, v := range generations {
		data[i] = []string{
			fmt.Sprintf("%v", v.Number),
			v.Description,
			fmt.Sprintf("%v", v.CreationDate.Format(time.ANSIC)),
		}
	}

	table := tablewriter.NewWriter(os.Stdout)

	table.SetHeader([]string{"#", "Description", "Creation Date"})
	table.SetHeaderAlignment(tablewriter.ALIGN_CENTER)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetAutoFormatHeaders(false)
	table.SetAutoWrapText(false)
	table.SetBorder(false)
	table.SetRowSeparator("-")
	table.SetColumnSeparator("|")
	table.AppendBulk(data)
	table.Render()
}

func deleteGenerations(s system.CommandRunner, profileDirectory string, generations []generation.Generation, verbose bool) error {
	argv := []string{"nix-env", "-p", profileDirectory, "--delete-generations"}
	for _, v := range generations {
		argv = append(argv, fmt.Sprintf("%v", v.Number))
	}

	if verbose {
		s.Logger().CmdArray(argv)
	}

	cmd := system.NewCommand(argv[0], argv[1:]...)
	_, err := s.Run(cmd)
	return err
}

func regenerateBootMenu(s system.CommandRunner, verbose bool) error {
	switchToConfiguration := filepath.Join(constants.CurrentSystem, "bin", "switch-to-configuration")

	argv := []string{switchToConfiguration, "boot"}

	if verbose {
		s.Logger().CmdArray(argv)
	}

	cmd := system.NewCommand(argv[0], argv[1:]...)
	_, err := s.Run(cmd)
	return err
}

func collectGarbage(s system.CommandRunner, verbose bool) error {
	var argv []string
	if build.Flake() {
		argv = []string{"nix", "store", "gc"}
	} else {
		argv = []string{"nix-collect-garbage"}
	}

	if verbose {
		argv = append(argv, "-v")
		s.Logger().CmdArray(argv)
	}

	var cmd *system.Command
	if len(argv) == 1 {
		cmd = system.NewCommand(argv[0])
	} else {
		cmd = system.NewCommand(argv[0], argv[1:]...)
	}

	_, err := s.Run(cmd)
	return err
}
