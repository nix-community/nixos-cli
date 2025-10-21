package generation

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"sort"
	"strconv"

	cmdUtils "github.com/nix-community/nixos-cli/internal/cmd/utils"
	"github.com/nix-community/nixos-cli/internal/constants"
	"github.com/spf13/cobra"
)

var genLinkRegex = regexp.MustCompile(`-(\d+)-link$`)

func CompleteProfileFlag(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	profiles := []string{"system"}

	entries, err := os.ReadDir(constants.NixSystemProfileDirectory)
	if err != nil {
		return []string{}, cobra.ShellCompDirectiveNoFileComp
	}

	for _, v := range entries {
		name := v.Name()

		if matches := genLinkRegex.FindStringSubmatch(name); len(matches) > 0 {
			continue
		}

		profiles = append(profiles, name)
	}

	sort.Strings(profiles)

	return profiles, cobra.ShellCompDirectiveNoFileComp
}

func CompleteGenerationNumber(profile *string, limit int) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		log, _ := cmdUtils.PrepareCompletionResources()

		if limit != 0 && len(args) >= limit {
			return []string{}, cobra.ShellCompDirectiveNoFileComp
		}

		generations, err := CollectGenerationsInProfile(log, *profile)
		if err != nil {
			return []string{}, cobra.ShellCompDirectiveNoFileComp
		}

		exclude := []uint64{}
		for _, v := range args {
			parsed, err := strconv.ParseUint(v, 10, 64)
			if err != nil {
				continue
			}
			exclude = append(exclude, parsed)
		}

		genNumbers := []string{}
		for _, v := range generations {
			if slices.Contains(exclude, v.Number) {
				continue
			}
			genNumber := fmt.Sprint(v.Number)
			if v.Description != "" {
				genNumber += "\t" + v.Description
			}
			genNumbers = append(genNumbers, genNumber)
		}

		sort.Strings(genNumbers)

		return genNumbers, cobra.ShellCompDirectiveNoFileComp
	}
}

func CompleteGenerationNumberFlag(profile *string) cobra.CompletionFunc {
	return func(cmd *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
		log, _ := cmdUtils.PrepareCompletionResources()

		generations, err := CollectGenerationsInProfile(log, *profile)
		if err != nil {
			return []string{}, cobra.ShellCompDirectiveNoFileComp
		}

		genNumbers := []string{}
		for _, v := range generations {
			genNumber := fmt.Sprint(v.Number)
			if v.Description != "" {
				genNumber += "\t" + v.Description
			}
			genNumbers = append(genNumbers, genNumber)
		}

		sort.Strings(genNumbers)

		return genNumbers, cobra.ShellCompDirectiveNoFileComp
	}
}
