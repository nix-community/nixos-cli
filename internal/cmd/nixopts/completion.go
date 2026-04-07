package nixopts

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type completionLine struct {
	Value       string
	Description string
}

// Attempt to collect completion values from the Nix
// command line completion facilities using NIX_GET_COMPLETIONS.
//
// Only works on modern Nix commands and internally sets
// NIX_CONFIG to facilitate this if necessary.
func collectNixCommandCompletionValues(argv []string, candidate string) ([]completionLine, error) {
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

	candidates := make([]completionLine, 0, len(lines))
	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 2)

		c := completionLine{
			Value: parts[0],
		}

		if len(parts) > 1 {
			c.Description = parts[1]
		}

		candidates = append(candidates, c)
	}

	return candidates, nil
}
