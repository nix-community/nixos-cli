package features

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/spf13/cobra"

	"github.com/nix-community/nixos-cli/internal/build"
)

func FeatureCommand() *cobra.Command {
	opts := cmdOpts.FeaturesOpts{}

	cmd := cobra.Command{
		Use:   "features",
		Short: "Show metadata about this application",
		Long:  "Show metadata about this application and configured options.",
		Run: func(cmd *cobra.Command, args []string) {
			featuresMain(cmd, &opts)
		},
	}

	cmdUtils.SetHelpFlagText(&cmd)

	cmd.Flags().BoolVarP(&opts.DisplayJson, "json", "j", false, "Output information in JSON format")

	return &cmd
}

type features struct {
	Version            string              `json:"version"`
	GitRevision        string              `json:"git_rev"`
	GoVersion          string              `json:"go_version"`
	DetectedNixVersion string              `json:"nix_version"`
	CompilationOptions complilationOptions `json:"options"`
}

type complilationOptions struct {
	NixpkgsVersion string `json:"nixpkgs_version"`
	Flake          bool   `json:"flake"`
}

func featuresMain(cmd *cobra.Command, opts *cmdOpts.FeaturesOpts) {
	log := logger.FromContext(cmd.Context())

	features := features{
		Version:     build.Version(),
		GitRevision: build.GitRevision(),
		GoVersion:   runtime.Version(),
		CompilationOptions: complilationOptions{
			NixpkgsVersion: build.NixpkgsVersion(),
			Flake:          build.Flake(),
		},
	}

	nixVersionCmd := exec.Command("nix", "--version")
	nixVersionOutput, _ := nixVersionCmd.Output()
	if nixVersionCmd.ProcessState.ExitCode() != 0 {
		log.Warn("nix version command failed to run, unable to detect nix version")
		features.DetectedNixVersion = "unknown"
	} else {
		features.DetectedNixVersion = strings.Trim(string(nixVersionOutput), "\n ")
	}

	if opts.DisplayJson {
		bytes, _ := json.MarshalIndent(features, "", "  ")
		fmt.Printf("%v\n", string(bytes))

		return
	}

	fmt.Printf("nixos %v\n", features.Version)
	fmt.Printf("git rev: %v\n", features.GitRevision)
	fmt.Printf("go version: %v\n", features.GoVersion)
	fmt.Printf("nix version: %v\n\n", features.DetectedNixVersion)

	fmt.Println("Compilation Options")
	fmt.Println("-------------------")

	fmt.Printf("flake           :: %v\n", features.CompilationOptions.Flake)
	fmt.Printf("nixpkgs_version :: %v\n", features.CompilationOptions.NixpkgsVersion)
}
