package nixopts

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

func constructFlakeInputCompletionFunc(resolver flakeRefResolver, addEqualsSign bool) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		resolvedFlakeRef, ok := resolver(cmd, args)
		if !ok {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		values, err := CollectNixCommandCompletionValues([]string{"build", resolvedFlakeRef, "--update-input"}, toComplete)
		if err != nil {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}

		directive := cobra.ShellCompDirectiveNoFileComp
		if addEqualsSign {
			directive |= cobra.ShellCompDirectiveNoSpace
		}

		candidates := make([]string, 0, len(values))
		for _, v := range values {
			formatString := "%s\t%s"
			if addEqualsSign {
				formatString = "%s=\t%s"
			}

			candidates = append(candidates, fmt.Sprintf(formatString, v.Value, v.Description))
		}

		return candidates, directive
	}
}

func collectOptionFlagCompletions(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	values, err := CollectNixCommandCompletionValues([]string{"--option"}, toComplete)
	if err != nil {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}

	if strings.Contains(toComplete, "=") {
		return nil, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
	}

	candidates := make([]string, 0, len(values))
	for _, v := range values {
		candidates = append(candidates, fmt.Sprintf("%s=\t%s", v.Value, v.Description))
	}

	return candidates, cobra.ShellCompDirectiveNoFileComp | cobra.ShellCompDirectiveNoSpace
}

type CompletionLine struct {
	Value       string
	Description string
}

// Attempt to collect completion values from the Nix
// command line completion facilities using NIX_GET_COMPLETIONS.
//
// Only works on modern Nix commands and internally sets
// NIX_CONFIG to facilitate this if necessary.
func CollectNixCommandCompletionValues(argv []string, candidate string) ([]CompletionLine, error) {
	nixCommandArgv := append([]string{"nix"}, argv...)
	nixCommandArgv = append(nixCommandArgv, candidate)

	// Index of candidate in the command array
	// will always be the last one.
	completionIndex := len(nixCommandArgv) - 1

	cmd := exec.Command(nixCommandArgv[0], nixCommandArgv[1:]...)

	// If 'nix-command' isn't enabled, then try to set it anyway.
	// This is just a safeguard considering NIX_CONFIG is not often
	// set.
	nixConfigEnv := strings.TrimSpace(os.Getenv("NIX_CONFIG"))
	if nixConfigEnv == "" {
		nixConfigEnv = "extra-experimental-features = nix-command flakes"
	} else if !strings.Contains(nixConfigEnv, "nix-command") {
		nixConfigEnv += "; extra-experimental-features = nix-command flakes"
	}

	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, fmt.Sprintf("NIX_GET_COMPLETIONS=%d", completionIndex))
	cmd.Env = append(cmd.Env, fmt.Sprintf("NIX_CONFIG=%s", nixConfigEnv))

	var stdout bytes.Buffer

	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	lines := strings.Split(strings.TrimSpace(stdout.String()), "\n")
	if len(lines) < 2 {
		return nil, nil
	}
	// Trim the first line, since that's usually a directive.
	lines = lines[1:]

	candidates := make([]CompletionLine, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 2)

		c := CompletionLine{
			Value: parts[0],
		}

		if len(parts) > 1 {
			c.Description = parts[1]
		}

		candidates = append(candidates, c)
	}

	return candidates, nil
}
