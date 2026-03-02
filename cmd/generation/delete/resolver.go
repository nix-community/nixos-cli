package delete

import (
	"fmt"
	"regexp"
	"slices"
	"sort"
	"time"

	"github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/generation"
)

type generationSet map[uint64]present

// evil type system hack to avoid typing struct{} all the time
type present struct{}

func resolveGenerationsToDelete(generations []generation.Generation, opts *cmdOpts.GenerationDeleteOpts) ([]generation.Generation, error) {
	currentGenIdx := slices.IndexFunc(generations, func(g generation.Generation) bool {
		return g.IsCurrent
	})
	if currentGenIdx == -1 {
		panic("current generation not found, this is a bug")
	}
	currentGen := generations[currentGenIdx]

	totalGenerations := uint64(len(generations))

	if totalGenerations == 0 {
		return nil, fmt.Errorf("no generations exist in profile")
	}
	if totalGenerations == 1 {
		return nil, fmt.Errorf("only one generations exists in profile, cannot delete the current generation")
	}

	if opts.MinimumToKeep > 0 && opts.MinimumToKeep >= totalGenerations {
		return nil, GenerationResolveMinError{ExpectedMinimum: opts.MinimumToKeep, AvailableGenerations: totalGenerations}
	}

	gensToKeep := make(generationSet, len(opts.Keep))
	for _, v := range opts.Keep {
		gensToKeep[uint64(v)] = present{}
	}
	gensToKeep[currentGen.Number] = present{}

	gensToRemove := make(generationSet, len(opts.Remove))
	for _, v := range opts.Remove {
		gensToRemove[uint64(v)] = present{}
	}

	if opts.All {
		for _, v := range generations {
			gensToRemove[v.Number] = present{}
		}
	} else {
		if opts.Pattern != "" {
			pattern := regexp.MustCompile(opts.Pattern)
			for _, g := range generations {
				if pattern.MatchString(g.Description) {
					gensToRemove[g.Number] = present{}
				}
			}
		}

		if opts.LowerBound != 0 || opts.UpperBound != 0 {
			upperBound := opts.UpperBound
			if upperBound == 0 {
				upperBound = generations[len(generations)-1].Number
			}
			lowerBound := opts.LowerBound
			if lowerBound == 0 {
				lowerBound = generations[0].Number
			}

			if lowerBound > upperBound {
				return nil, GenerationResolveBoundsError{LowerBound: lowerBound, UpperBound: upperBound}
			}
			if upperBound > generations[len(generations)-1].Number || upperBound < generations[0].Number {
				return nil, GenerationResolveRangeError{InvalidBound: upperBound}
			}
			if lowerBound < generations[0].Number || lowerBound > generations[len(generations)-1].Number {
				return nil, GenerationResolveRangeError{InvalidBound: lowerBound}
			}

			for _, v := range generations {
				if v.Number >= lowerBound && v.Number <= upperBound {
					gensToRemove[v.Number] = present{}
				}
			}
		}

		if opts.OlderThan > 0 {
			upperDateBound := time.Now().Add(-opts.OlderThan.Duration())

			for _, v := range generations {
				if v.CreationDate.Before(upperDateBound) {
					gensToRemove[v.Number] = present{}
				}
			}
		}
	}

	for g := range gensToKeep {
		delete(gensToRemove, g)
	}

	remainingGenCount := uint64(len(generations) - len(gensToRemove))
	if opts.MinimumToKeep > 0 && remainingGenCount < opts.MinimumToKeep {
		for j := range generations {
			i := len(generations) - 1 - j
			g := generations[i]

			delete(gensToRemove, g.Number)

			remainingGenCount = uint64(len(generations) - len(gensToRemove))
			if remainingGenCount == opts.MinimumToKeep {
				break
			}
		}
	}

	if len(gensToRemove) == 0 {
		return nil, GenerationResolveNoneFoundError{}
	}

	result := make([]generation.Generation, 0, len(gensToRemove))
	for num := range gensToRemove {
		for _, g := range generations {
			if g.Number == num {
				result = append(result, g)
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Number < result[j].Number
	})
	return result, nil
}

type GenerationResolveMinError struct {
	ExpectedMinimum      uint64
	AvailableGenerations uint64
}

func (e GenerationResolveMinError) Error() string {
	return fmt.Sprintf("cannot keep %v generations, there are only %v available", e.ExpectedMinimum, e.AvailableGenerations)
}

type GenerationResolveBoundsError struct {
	LowerBound uint64
	UpperBound uint64
}

func (e GenerationResolveBoundsError) Error() string {
	return fmt.Sprintf("lower bound '%v' must be less than upper bound '%v'", e.LowerBound, e.UpperBound)
}

type GenerationResolveRangeError struct {
	InvalidBound uint64
}

func (e GenerationResolveRangeError) Error() string {
	return fmt.Sprintf("bound '%v' is not within the range of available generations", e.InvalidBound)
}

type GenerationResolveNoneFoundError struct{}

func (e GenerationResolveNoneFoundError) Error() string {
	return "no generations were resolved for deletion from the given parameters"
}
