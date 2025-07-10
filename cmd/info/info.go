package info

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/fatih/color"
	"github.com/nix-community/nixos-cli/internal/activation"
	"github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/system"
	"github.com/spf13/cobra"
)

func InfoCommand() *cobra.Command {
	opts := cmdOpts.InfoOpts{}

	cmd := cobra.Command{
		Use:   "info",
		Short: "Show info about the currently running generation",
		Long:  "Show information about the currently running NixOS generation.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmdUtils.CommandErrorHandler(infoMain(cmd, &opts))
		},
	}

	cmdUtils.SetHelpFlagText(&cmd)

	cmd.Flags().BoolVarP(&opts.DisplayJson, "json", "j", false, "Format output as JSON")
	cmd.Flags().BoolVarP(&opts.DisplayMarkdown, "markdown", "m", false, "Format output as Markdown for reporting")

	return &cmd
}

const (
	markdownTemplate = `- nixos version: %v
- nixpkgs revision: %v
- kernel version: %v
`
)

func infoMain(cmd *cobra.Command, opts *cmdOpts.InfoOpts) error {
	log := logger.FromContext(cmd.Context())
	s := system.NewLocalSystem(log)

	if !s.IsNixOS() {
		msg := "the info command is only supported on NixOS systems"
		log.Errorf(msg)
		return fmt.Errorf("%v", msg)
	}

	// Only support the `system` profile for now.
	currentGenNumber, err := activation.GetCurrentGenerationNumber("system")
	if err != nil {
		log.Warnf("failed to determine current generation number: %v", err)
		return err
	}

	currentGen, err := generation.GenerationFromDirectory(constants.CurrentSystem, currentGenNumber)
	if err != nil {
		log.Warnf("failed to collect generations: %v", err)
		return err
	}
	currentGen.Number = currentGenNumber
	currentGen.IsCurrent = true

	if opts.DisplayJson {
		bytes, _ := json.MarshalIndent(currentGen, "", "  ")
		fmt.Printf("%v\n", string(bytes))
		return nil
	}

	if opts.DisplayMarkdown {
		fmt.Printf(markdownTemplate, currentGen.NixosVersion, currentGen.NixpkgsRevision, currentGen.KernelVersion)
		return nil
	}

	prettyPrintGenInfo(currentGen)

	return nil
}

var titleColor = color.New(color.Bold, color.Italic)

func prettyPrintGenInfo(g *generation.Generation) {
	version := g.NixosVersion
	if version == "" {
		version = "NixOS (unknown version)"
	}

	titleColor.Printf("%v\n", version)
	titleColor.Println(strings.Repeat("-", len(version)))

	printKey("Generation")
	fmt.Println(g.Number)

	printKey("Description")
	desc := g.Description
	if desc == "" {
		desc = color.New(color.Italic).Sprint("(none)")
	}
	fmt.Println(desc)

	printKey("Nixpkgs Version")
	nixpkgsVersion := g.NixpkgsRevision
	if nixpkgsVersion == "" {
		nixpkgsVersion = color.New(color.Italic).Sprint("(unknown)")
	}
	fmt.Println(nixpkgsVersion)

	printKey("Config Version")
	configVersion := g.ConfigurationRevision
	if configVersion == "" {
		configVersion = color.New(color.Italic).Sprint("(unknown)")
	}
	fmt.Println(configVersion)

	printKey("Kernel Version")
	kernelVersion := g.KernelVersion
	if kernelVersion == "" {
		kernelVersion = color.New(color.Italic).Sprint("(unknown)")
	}
	fmt.Println(kernelVersion)

	printKey("Specialisations")
	specialisations := strings.Join(g.Specialisations, ", ")
	if specialisations == "" {
		specialisations = color.New(color.Italic).Sprint("(none)")
	}
	fmt.Println(specialisations)
}

func getKeyMaxLength() int {
	strings := []string{
		"Generation", "Description", "NixOS Version", "Nixpkgs Version",
		"Config Version", "Kernel Version", "Specialisations",
	}

	maxLength := 0

	for _, v := range strings {
		l := len(color.CyanString(v))
		if l > maxLength {
			maxLength = l
		}
	}

	return maxLength
}

func printKey(key string) {
	fmt.Printf("%-"+fmt.Sprintf("%v", keyMaxLength)+"v :: ", color.CyanString(key))
}

var keyMaxLength = getKeyMaxLength()
