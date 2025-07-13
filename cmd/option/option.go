package option

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"slices"

	"github.com/nix-community/nixos-cli/internal/build"
	"github.com/nix-community/nixos-cli/internal/cmd/nixopts"
	"github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/configuration"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/sahilm/fuzzy"
	"github.com/spf13/cobra"
	"github.com/water-sucks/optnix/option"
	optionTUI "github.com/water-sucks/optnix/tui"
	"github.com/yarlson/pin"
)

func OptionCommand() *cobra.Command {
	opts := cmdOpts.OptionOpts{}

	cmd := cobra.Command{
		Use:   "option [flags] [NAME]",
		Short: "Query NixOS options and their details",
		Long:  "Query available NixOS module options for this system.",
		Args: func(cmd *cobra.Command, args []string) error {
			argsFunc := cobra.ExactArgs(1)
			if opts.Interactive {
				argsFunc = cobra.MaximumNArgs(1)
			}

			if err := argsFunc(cmd, args); err != nil {
				return err
			}

			if len(args) > 0 {
				opts.OptionInput = args[0]
			}

			return nil
		},
		ValidArgsFunction: OptionsCompletionFunc(&opts),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdUtils.CommandErrorHandler(optionMain(cmd, &opts))
		},
	}

	cmd.Flags().BoolVarP(&opts.DisplayJson, "json", "j", false, "Output information in JSON format")
	cmd.Flags().BoolVarP(&opts.Interactive, "interactive", "i", false, "Show interactive search TUI for options")
	cmd.Flags().BoolVarP(&opts.NoUseCache, "no-cache", "n", false, "Do not attempt to use prebuilt option cache")
	cmd.Flags().Int64VarP(&opts.MinScore, "min-score", "s", 0, "")
	cmd.Flags().BoolVarP(&opts.DisplayValueOnly, "value-only", "v", false, "Show only the selected option's value")

	if build.Flake() {
		cmd.Flags().StringVarP(&opts.FlakeRef, "flake", "f", "", "Flake ref to explicitly load options from")
	}

	nixopts.AddIncludesNixOption(&cmd, &opts.NixPathIncludes)

	cmd.MarkFlagsMutuallyExclusive("json", "interactive", "value-only")

	cmdUtils.SetHelpFlagText(&cmd)
	cmd.SetHelpTemplate(cmd.HelpTemplate() + `
Arguments:
  [NAME]  Name of option to use. Not required in interactive mode.
`)

	return &cmd
}

func optionMain(cmd *cobra.Command, opts *cmdOpts.OptionOpts) error {
	log := logger.FromContext(cmd.Context())
	cfg := settings.FromContext(cmd.Context())
	s := system.NewLocalSystem(log)

	minScore := cfg.Option.MinScore
	if cmd.Flags().Changed("min-score") {
		minScore = opts.MinScore
	}

	if !s.IsNixOS() {
		msg := "this command is only supported on NixOS systems"
		log.Error(msg)
		return fmt.Errorf("%v", msg)
	}

	var nixosConfig configuration.Configuration
	if opts.FlakeRef != "" {
		nixosConfig = configuration.FlakeRefFromString(opts.FlakeRef)
		if err := nixosConfig.(*configuration.FlakeRef).InferSystemFromHostnameIfNeeded(); err != nil {
			log.Errorf("failed to infer hostname: %v", err)
			return err
		}
	} else {
		c, err := configuration.FindConfiguration(log, cfg, opts.NixPathIncludes, false)
		if err != nil {
			log.Errorf("failed to find configuration: %v", err)
			return err
		}
		nixosConfig = c
	}

	spinner := pin.New("Loading...",
		pin.WithSpinnerColor(pin.ColorCyan),
		pin.WithTextColor(pin.ColorRed),
		pin.WithPosition(pin.PositionRight),
		pin.WithSpinnerFrames([]rune{'-', '\\', '|', '/'}),
		pin.WithWriter(os.Stderr),
	)
	cancelSpinner := spinner.Start(context.Background())
	defer cancelSpinner()

	spinner.UpdateMessage("Loading options...")

	useCache := !opts.NoUseCache
	if useCache {
		_, err := os.Stat(prebuiltOptionCachePath)
		if err != nil {
			log.Warnf("error accessing prebuilt option cache: %v", err)
			useCache = false
		}
	}

	optionsFileName := prebuiltOptionCachePath
	if !useCache {
		f, err := buildOptionCache(s, nixosConfig)
		if err != nil {
			spinner.Stop()
			log.Errorf("failed to build option list: %v", err)
			log.Errorf("evaluation trace:", f)
			return err
		}
		optionsFileName = f
	}

	optionsFile, err := os.Open(optionsFileName)
	if err != nil {
		log.Errorf("failed to open options file %v: %v", optionsFileName, err)
		return err
	}

	options, err := option.LoadOptions(optionsFile)
	if err != nil {
		spinner.Stop()
		log.Errorf("failed to load options: %v", err)
		return err
	}

	var evaluator option.EvaluatorFunc = func(optionName string) (string, error) {
		value, err := nixosConfig.EvalAttribute(optionName)
		realValue := ""
		if value != nil {
			realValue = *value
		}
		return realValue, err
	}

	if opts.Interactive {
		spinner.Stop()
		return optionTUI.OptionTUI(optionTUI.OptionTUIArgs{
			Options:      options,
			ScopeName:    "nixos",
			MinScore:     minScore,
			DebounceTime: cfg.Option.DebounceTime,
			Evaluator:    evaluator,
			InitialInput: opts.OptionInput,
		})
	}

	spinner.UpdateMessage(fmt.Sprintf("Finding option %v...", opts.OptionInput))

	exactOptionMatchIdx := slices.IndexFunc(options, func(o option.NixosOption) bool {
		return o.Name == opts.OptionInput
	})
	if exactOptionMatchIdx != -1 {
		o := options[exactOptionMatchIdx]

		spinner.UpdateMessage("Evaluating option value...")

		evaluatedValue, err := evaluator(o.Name)

		spinner.Stop()

		if opts.DisplayJson {
			displayOptionJson(&o, evaluatedValue)
		} else if opts.DisplayValueOnly {
			fmt.Printf("%v\n", evaluatedValue)
		} else {
			fmt.Print(o.PrettyPrint(&option.ValuePrinterInput{
				Value: evaluatedValue,
				Err:   err,
			}))
		}

		return nil
	}

	spinner.Stop()

	msg := fmt.Sprintf("no exact match for query '%s' found", opts.OptionInput)
	err = fmt.Errorf("%v", msg)

	fuzzySearchResults := fuzzy.FindFrom(opts.OptionInput, options)
	if len(fuzzySearchResults) > 10 {
		fuzzySearchResults = fuzzySearchResults[:10]
	}

	fuzzySearchResults = filterMinimumScoreMatches(fuzzySearchResults, int(minScore))

	if opts.DisplayJson {
		displayErrorJson(msg, fuzzySearchResults)
		return err
	}

	log.Error(msg)
	if len(fuzzySearchResults) > 0 {
		log.Print("\nSome similar options were found:\n")
		for _, v := range fuzzySearchResults {
			log.Printf(" - %s\n", v.Str)
		}
	} else {
		log.Print("\nTry refining your search query.\n")
	}

	return err
}

func displayOptionJson(o *option.NixosOption, evaluatedValue string) {
	type optionJson struct {
		Name         string   `json:"name"`
		Description  string   `json:"description"`
		Type         string   `json:"type"`
		Value        string   `json:"value"`
		Default      string   `json:"default"`
		Example      string   `json:"example"`
		Location     []string `json:"loc"`
		ReadOnly     bool     `json:"readOnly"`
		Declarations []string `json:"declarations"`
	}

	defaultText := ""
	if o.Default != nil {
		defaultText = o.Default.Text
	}

	exampleText := ""
	if o.Example != nil {
		exampleText = o.Example.Text
	}

	bytes, _ := json.MarshalIndent(optionJson{
		Name:         o.Name,
		Description:  o.Description,
		Type:         o.Type,
		Value:        evaluatedValue,
		Default:      defaultText,
		Example:      exampleText,
		Location:     o.Location,
		ReadOnly:     o.ReadOnly,
		Declarations: o.Declarations,
	}, "", "  ")
	fmt.Printf("%v\n", string(bytes))
}

func displayErrorJson(msg string, matches fuzzy.Matches) {
	type errorJson struct {
		Message        string   `json:"message"`
		SimilarOptions []string `json:"similar_options"`
	}

	matchedStrings := make([]string, len(matches))
	for i, match := range matches {
		matchedStrings[i] = match.Str
	}

	bytes, _ := json.MarshalIndent(errorJson{
		Message:        msg,
		SimilarOptions: matchedStrings,
	}, "", "  ")
	fmt.Printf("%v\n", string(bytes))
}

// Filter a sorted (descending) match list until a minimum score is reached.
// Return a slice of the original matches.
func filterMinimumScoreMatches(matches []fuzzy.Match, minScore int) []fuzzy.Match {
	for i, v := range matches {
		if v.Score < minScore {
			return matches[:i]
		}
	}

	return matches
}
