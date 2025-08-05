package completion

import (
	"fmt"

	"github.com/carapace-sh/carapace"
	"github.com/spf13/cobra"
)

func CompletionCommand() *cobra.Command {
	cmd := cobra.Command{
		Use:                   "completion {bash|zsh|fish}",
		Short:                 "Generate completion scripts",
		Long:                  "Generate completion scripts for use in shells.",
		Hidden:                true,
		DisableFlagsInUseLine: true,
		ValidArgs:             []string{},
		Args:                  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		Run: func(cmd *cobra.Command, args []string) {
			completion_script, err := carapace.Gen(cmd.Root()).Snippet(args[0])
			if err != nil {
				fmt.Println("", err)
				return
			}
			fmt.Println(completion_script)
		},
	}

	return &cmd
}
