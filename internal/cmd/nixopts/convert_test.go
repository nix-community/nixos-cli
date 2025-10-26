package nixopts_test

import (
	"reflect"
	"testing"

	"github.com/nix-community/nixos-cli/internal/cmd/nixopts"
	"github.com/spf13/cobra"
)

type nixOptions struct {
	Quiet          bool              `nixCategory:"build,copy"`
	PrintBuildLogs bool              `nixCategory:"build"`
	MaxJobs        int               `nixCategory:"build,copy"`
	LogFormat      string            `nixCategory:"build"`
	Builders       []string          `nixCategory:"build"`
	Options        map[string]string `nixCategory:"build,eval"`
}

func createTestCmd() (*cobra.Command, *nixOptions) {
	opts := nixOptions{}

	cmd := &cobra.Command{}

	nixopts.AddQuietNixOption(cmd, &opts.Quiet)
	nixopts.AddPrintBuildLogsNixOption(cmd, &opts.PrintBuildLogs)
	nixopts.AddMaxJobsNixOption(cmd, &opts.MaxJobs)
	nixopts.AddLogFormatNixOption(cmd, &opts.LogFormat)
	nixopts.AddBuildersNixOption(cmd, &opts.Builders)
	nixopts.AddOptionNixOption(cmd, &opts.Options)

	return cmd, &opts
}

func TestNixOptionsToArgsList(t *testing.T) {
	tests := []struct {
		name string
		// The command-line arguments passed to Cobra
		passedArgs []string
		// The expected arguments to be passed to Nix
		expected []string
	}{
		{
			name:       "All fields zero-valued",
			passedArgs: []string{},
			expected:   []string{},
		},
		{
			name:       "Single boolean field",
			passedArgs: []string{"--quiet"},
			expected:   []string{"--quiet"},
		},
		{
			name:       "Integer field set",
			passedArgs: []string{"--max-jobs", "4"},
			expected:   []string{"--max-jobs", "4"},
		},
		{
			name:       "Integer field set to zero value",
			passedArgs: []string{"--max-jobs", "0"},
			expected:   []string{"--max-jobs", "0"},
		},
		{
			name:       "String field set",
			passedArgs: []string{"--log-format", "json"},
			expected:   []string{"--log-format", "json"},
		},
		{
			name:       "Slice field set",
			passedArgs: []string{"--builders", "builder1", "--builders", "builder2"},
			expected:   []string{"--builders", "builder1", "--builders", "builder2"},
		},
		{
			name:       "Map field set",
			passedArgs: []string{"--option", "option1=value1", "--option", "option2=value2"},
			expected:   []string{"--option", "option1", "value1", "--option", "option2", "value2"},
		},
		{
			name:       "Mixed fields set",
			passedArgs: []string{"--quiet", "--max-jobs", "2", "--log-format", "xml", "--builders", "builder1", "--option", "option1=value1", "--option", "option2=value2"},
			expected:   []string{"--quiet", "--max-jobs", "2", "--log-format", "xml", "--builders", "builder1", "--option", "option1", "value1", "--option", "option2", "value2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, opts := createTestCmd()

			// Dummy execution of "command" for Cobra to parse flags
			cmd.SetArgs(tt.passedArgs)
			_ = cmd.Execute()

			args := nixopts.NixOptionsToArgsList(cmd.Flags(), opts)

			if !reflect.DeepEqual(args, tt.expected) {
				t.Errorf("NixOptionsToArgsList() = %v, want %v", args, tt.expected)
			}
		})
	}
}

func TestNixOptionsToArgsListByCategory(t *testing.T) {
	tests := []struct {
		name       string
		category   string
		passedArgs []string
		expected   []string
	}{
		{
			name:       "Build category only includes build-related flags",
			category:   "build",
			passedArgs: []string{"--quiet", "--max-jobs", "2", "--log-format", "json", "--builders", "builder1"},
			expected:   []string{"--quiet", "--max-jobs", "2", "--log-format", "json", "--builders", "builder1"},
		},
		{
			name:       "Eval category only includes eval-related flags",
			category:   "eval",
			passedArgs: []string{"--option", "foo=bar", "--quiet"},
			expected:   []string{"--option", "foo", "bar"},
		},
		{
			name:       "Copy category only includes copy-related flags",
			category:   "copy",
			passedArgs: []string{"--quiet", "--max-jobs", "3"},
			expected:   []string{"--quiet", "--max-jobs", "3"},
		},
		{
			name:       "Category with no matching flags returns empty slice",
			category:   "lock",
			passedArgs: []string{"--quiet", "--max-jobs", "3"},
			expected:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, opts := createTestCmd()
			cmd.SetArgs(tt.passedArgs)
			_ = cmd.Execute()

			args := nixopts.NixOptionsToArgsListByCategory(cmd.Flags(), opts, tt.category)
			if !reflect.DeepEqual(args, tt.expected) {
				t.Errorf("NixOptionsToArgsListByCategory(%s) = %v, want %v", tt.category, args, tt.expected)
			}
		})
	}
}
