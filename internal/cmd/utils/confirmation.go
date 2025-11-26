package cmdUtils

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/fatih/color"
	"github.com/nix-community/nixos-cli/internal/settings"
)

type ConfirmationPromptOptions struct {
	InvalidBehavior settings.ConfirmationPromptBehavior
	EmptyBehavior   settings.ConfirmationPromptBehavior
}

func ConfirmationInput(msg string, opts ConfirmationPromptOptions) (bool, error) {
	var input string
	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Fprintf(os.Stderr, "%s\n[y/n]: ", color.GreenString("|> %s", msg))

		_ = scanner.Scan()
		input = scanner.Text()
		if err := scanner.Err(); err != nil {
			return false, err
		}

		input = strings.ToLower(strings.TrimSpace(input))

		if len(input) == 0 {
			switch opts.EmptyBehavior {
			case settings.ConfirmationPromptRetry:
				fmt.Fprintln(os.Stderr, "error: input must not be empty")
				continue
			case settings.ConfirmationPromptDefaultNo:
				fmt.Fprintln(os.Stderr, "no input provided; defaulting to no")
				return false, nil
			case settings.ConfirmationPromptDefaultYes:
				fmt.Fprintln(os.Stderr, "no input provided; defaulting to yes")
				return true, nil
			default:
				return false, fmt.Errorf("unhandled EmptyBehavior case: %s", opts.EmptyBehavior)
			}
		}

		switch input[0] {
		case 'y':
			return true, nil
		case 'n':
			return false, nil
		}

		switch opts.InvalidBehavior {
		case settings.ConfirmationPromptRetry:
			fmt.Fprintf(os.Stderr, "error: invalid input '%s'; must be y/n\n", input)
			continue
		case settings.ConfirmationPromptDefaultNo:
			fmt.Fprintf(os.Stderr, "warning: invalid input '%s'; defaulting to no\n", input)
			return false, nil
		case settings.ConfirmationPromptDefaultYes:
			fmt.Fprintf(os.Stderr, "warning: invalid input '%s'; defaulting to yes\n", input)
			return true, nil
		default:
			return false, fmt.Errorf("unhandled InvalidBehavior case: %s", opts.InvalidBehavior)
		}
	}
}
