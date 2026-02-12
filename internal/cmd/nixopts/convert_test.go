package nixopts_test

import (
	"reflect"
	"testing"

	"github.com/nix-community/nixos-cli/internal/cmd/nixopts"
	"github.com/spf13/cobra"
)

type nixOptions struct {
	nixopts.Quiet
	nixopts.PrintBuildLogs
	nixopts.SubstituteOnDestination
	nixopts.MaxJobs
	nixopts.LogFormat
	nixopts.Include
	nixopts.Option
}

func (o *nixOptions) Flags() []nixopts.NixOption {
	return nixopts.CollectFlags(o)
}

func (o *nixOptions) ArgsForCommand(cmd nixopts.NixCommand) []string {
	return nixopts.ArgsForOptionsSet(o.Flags(), cmd)
}

func createTestCmd() (*cobra.Command, nixopts.NixOptionsSet) {
	opts := nixOptions{}

	cmd := &cobra.Command{}

	for _, opt := range opts.Flags() {
		opt.Bind(cmd)
	}

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
			expected:   nil,
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
			passedArgs: []string{"--include", "include1", "--include", "include2"},
			expected:   []string{"--include", "include1", "--include", "include2"},
		},
		{
			name:       "Map field set",
			passedArgs: []string{"--option", "option1=value1", "--option", "option2=value2"},
			expected:   []string{"--option", "option1", "value1", "--option", "option2", "value2"},
		},
		{
			name:       "Mixed fields set",
			passedArgs: []string{"--quiet", "--max-jobs", "2", "--log-format", "xml", "--include", "include1", "--option", "option1=value1", "--option", "option2=value2"},
			expected:   []string{"--include", "include1", "--log-format", "xml", "--max-jobs", "2", "--option", "option1", "value1", "--option", "option2", "value2", "--quiet"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, opts := createTestCmd()

			// Dummy execution of "command" for Cobra to parse flags
			cmd.SetArgs(tt.passedArgs)
			_ = cmd.Execute()

			args := opts.ArgsForCommand(nixopts.CmdBuild)

			if !reflect.DeepEqual(args, tt.expected) {
				t.Errorf("NixOptionsToArgsList() = %v, want %v", args, tt.expected)
			}
		})
	}
}

func TestNixOptionsToArgsListByCommand(t *testing.T) {
	tests := []struct {
		name       string
		command    nixopts.NixCommand
		passedArgs []string
		expected   []string
	}{
		{
			name:       "Filter build options category",
			command:    nixopts.CmdBuild,
			passedArgs: []string{"--quiet", "--max-jobs", "2", "--log-format", "json", "--include", "include1", "--substitute-on-destination"},
			expected:   []string{"--include", "include1", "--log-format", "json", "--max-jobs", "2", "--quiet"},
		},
		{
			name:       "Filter eval options category",
			command:    nixopts.CmdEval,
			passedArgs: []string{"--option", "foo=bar", "--quiet", "--substitute-on-destination"},
			expected:   []string{"--option", "foo", "bar", "--quiet"},
		},
		{
			name:       "Filter copy options category",
			command:    nixopts.CmdCopyClosure,
			passedArgs: []string{"--quiet", "--max-jobs", "3", "--substitute-on-destination"},
			expected:   []string{"--max-jobs", "3", "--quiet", "-s"},
		},
		{
			name:       "Category with no matching flags returns nil",
			command:    "nonexistent",
			passedArgs: []string{"--quiet", "--max-jobs", "3"},
			expected:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd, opts := createTestCmd()
			cmd.SetArgs(tt.passedArgs)
			_ = cmd.Execute()

			args := opts.ArgsForCommand(tt.command)
			if !reflect.DeepEqual(args, tt.expected) {
				t.Errorf("NixOptionsToArgsListByCommand(%s) = %v, want %v", tt.command, args, tt.expected)
			}
		})
	}
}
