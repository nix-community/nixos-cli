package cmdUtils

import (
	"errors"
	"fmt"
	"maps"
	"os"
	"slices"
	"sort"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
)

func SetHelpFlagText(cmd *cobra.Command) {
	cmd.Flags().BoolP("help", "h", false, "Show this help menu")
}

var ErrCommand = errors.New("command error")

// Replace a returned error with the generic `ErrCommand`, and.
// exit with a non-zero exit code. This is to avoid extra error
// messages being printed when a command function defined with
// RunE returns a non-nil error.
func CommandErrorHandler(err error) error {
	if err != nil {
		os.Exit(1)
		return ErrCommand
	}
	return nil
}

func ConfigureBubbleTeaLogger(prefix string) (func(), error) {
	if os.Getenv("NIXOS_CLI_DEBUG_MODE") == "" {
		return func() {}, nil
	}

	file, err := tea.LogToFile("debug.log", prefix)

	return func() {
		if err != nil || file == nil {
			return
		}
		_ = file.Close()
	}, err
}

func AlignedOptions(options map[string]string) string {
	maxLen := 0
	for cmd := range options {
		if len(cmd) > maxLen {
			maxLen = len(cmd)
		}
	}

	result := ""
	format := fmt.Sprintf("  %%-%ds  %%s\n", maxLen)

	keys := slices.Collect(maps.Keys(options))
	sort.Strings(keys)

	for _, cmd := range keys {
		desc := options[cmd]
		result += fmt.Sprintf(format, cmd, desc)
	}

	return result
}
