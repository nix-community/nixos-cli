package delete

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/nix-community/nixos-cli/internal/cmd/opts"
	"github.com/nix-community/nixos-cli/internal/generation"
	systemdUtils "github.com/nix-community/nixos-cli/internal/systemd"
)

func TestResolveGenerationsToDelete(t *testing.T) {
	timeNow := time.Now()
	generations := []generation.Generation{
		{Number: 1, CreationDate: timeNow.Add(-48 * time.Hour), IsCurrent: false},
		{Number: 2, CreationDate: timeNow.Add(-24 * time.Hour), IsCurrent: false},
		{Number: 3, CreationDate: timeNow, IsCurrent: true},
	}

	tests := []struct {
		name      string
		opts      *cmdOpts.GenerationDeleteOpts
		expect    []uint64
		expectErr error
	}{
		{
			name: "Delete all generations",
			opts: &cmdOpts.GenerationDeleteOpts{
				All: true,
			},
			expect: []uint64{1, 2},
		},
		{
			name: "Keep specific generations",
			opts: &cmdOpts.GenerationDeleteOpts{
				Keep: []uint{1},
				All:  true,
			},
			expect: []uint64{2},
		},
		{
			name: "Minimum to keep",
			opts: &cmdOpts.GenerationDeleteOpts{
				MinimumToKeep: 3,
			},
			expect: []uint64{},
			expectErr: GenerationResolveMinError{
				ExpectedMinimum:      3,
				AvailableGenerations: 3,
			},
		},
		{
			name: "Lower and upper bounds",
			opts: &cmdOpts.GenerationDeleteOpts{
				LowerBound: 1,
				UpperBound: 2,
			},
			expect: []uint64{1, 2},
		},
		{
			name: "Older than specified duration",
			opts: &cmdOpts.GenerationDeleteOpts{
				OlderThan: systemdUtils.SystemdDuration(24 * time.Hour),
			},
			expect: []uint64{1, 2},
		},
		{
			name: "Remove specific generations",
			opts: &cmdOpts.GenerationDeleteOpts{
				Remove: []uint{1},
			},
			expect: []uint64{1},
		},
		{
			name: "Invalid lower and upper bounds",
			opts: &cmdOpts.GenerationDeleteOpts{
				LowerBound: 3,
				UpperBound: 1,
			},
			expectErr: GenerationResolveBoundsError{LowerBound: 3, UpperBound: 1},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := resolveGenerationsToDelete(generations, test.opts)

			if test.expectErr != nil {
				if !errors.Is(err, test.expectErr) {
					t.Errorf("expected error %v, got %v", test.expectErr, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}

				resultNumbers := make([]uint64, len(result))
				for i, g := range result {
					resultNumbers[i] = g.Number
				}

				if !reflect.DeepEqual(test.expect, resultNumbers) {
					t.Errorf("expected %v, got %v", test.expect, resultNumbers)
				}
			}
		})
	}
}
