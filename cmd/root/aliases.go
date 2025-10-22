package root

import (
	"fmt"
	"os"

	"github.com/carapace-sh/carapace"
	"github.com/nix-community/nixos-cli/internal/utils"
	"github.com/spf13/cobra"
)

func addAliasCmd(parent *cobra.Command, alias string, args []string) error {
	displayedArgs := utils.EscapeAndJoinArgs(args)
	description := fmt.Sprintf("Alias for `%v`.", displayedArgs)

	existingCommands := parent.Commands()
	for _, v := range existingCommands {
		if v.Name() == alias {
			return fmt.Errorf("alias conflicts with existing builtin command")
		}
	}

	if !parent.ContainsGroup("aliases") {
		parent.AddGroup(&cobra.Group{
			ID:    "aliases",
			Title: "Aliases",
		})
	}

	cmd := &cobra.Command{
		Use:                alias,
		Short:              description,
		Long:               description,
		GroupID:            "aliases",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, passedArgs []string) error {
			fullArgsList := append(args, passedArgs...)

			root := cmd.Root()
			root.SetArgs(fullArgsList)
			return root.Execute()
		},
	}

	parent.AddCommand(cmd)

	carapace.Gen(cmd).PositionalAnyCompletion(
		carapace.ActionCallback(
			func(c carapace.Context) carapace.Action {
				// HACK: So this is a rather lazy way of implementing completion for aliases.
				// I couldn't figure out how to get completions from the flag, so I decided
				// to just run the hidden completion command with the resolved arguments
				// and anything else that was passed. This should be negligible from a
				// performance perspective, but it's definitely a piece of shit.
				// Also, if you know, you know.

				// evil completion command hacking
				completionArgv := []string{os.Args[0], "_carapace", "export", ""} // what the fuck?
				completionArgv = append(completionArgv, args...)
				completionArgv = append(completionArgv, c.Args...)
				completionArgv = append(completionArgv, c.Value)

				return carapace.ActionExecCommand(completionArgv[0], completionArgv[1:]...)(func(output []byte) carapace.Action {
					if string(output) == "" {
						return carapace.ActionValues()
					}
					return carapace.ActionImport(output)
				})
			},
		),
	)

	return nil
}
