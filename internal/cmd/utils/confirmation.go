package cmdUtils

import (
	"bufio"
	"context"
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

// This operation can be cancelled using the provided context,
// but any errors returned from here MAY have the potential
// to keep consuming stdin until another character is typed
// if a duplicate instance of stdin cannot be opened, so
// any errors here will result in potentially undefined behavior
// for stdin input.
func ConfirmationInput(ctx context.Context, msg string, opts ConfirmationPromptOptions) (bool, error) {
	var stdin *os.File
	if dupStdin, err := os.OpenFile("/dev/stdin", os.O_RDONLY, 0); err == nil {
		defer dupStdin.Close()
		stdin = dupStdin
	} else {
		// NOTE: falling back to stdin will make context
		// cancellation behavior a bit unclear, as mentioned
		// in the doc comment.
		stdin = os.Stdin
	}

	var input string
	scanner := bufio.NewScanner(stdin)

	inputCh := make(chan string)
	errCh := make(chan error, 1)

	go func() {
		defer close(inputCh)

		for scanner.Scan() {
			select {
			case inputCh <- scanner.Text():
			case <-ctx.Done():
				return
			}
		}

		if err := scanner.Err(); err != nil {
			select {
			case errCh <- err:
			default:
			}
		}
	}()

	for {
		fmt.Fprintf(os.Stderr, "%s\n[y/n]: ", color.GreenString("|> %s", msg))

		var ok bool
		var err error

		select {
		case <-ctx.Done():
			return false, ctx.Err()

		case err, ok = <-errCh:
			if !ok {
				return false, nil
			}

			if err != nil {
				return false, err
			}
			return false, nil

		case input, ok = <-inputCh:
			if !ok {
				return false, nil
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
}
