package genUtils

import (
	"github.com/nix-community/nixos-cli/internal/generation"
	"github.com/nix-community/nixos-cli/internal/logger"
	"github.com/nix-community/nixos-cli/internal/system"
)

func LoadGenerations(log logger.Logger, profileName string, reverse bool) ([]generation.Generation, error) {
	s := system.NewLocalSystem(log)

	generations, err := generation.CollectGenerationsInProfile(s, log, profileName)
	if err != nil {
		switch v := err.(type) {
		case *generation.GenerationReadError:
			for _, err := range v.Errors {
				log.Warnf("%v", err)
			}

		default:
			log.Errorf("error collecting generation information: %v", v)
			return nil, v
		}
	}

	if reverse {
		for i, j := 0, len(generations)-1; i < j; i, j = i+1, j-1 {
			generations[i], generations[j] = generations[j], generations[i]
		}
	}

	return generations, nil
}
