package settings_test

import (
	"testing"

	"github.com/nix-community/nixos-cli/internal/settings"
)

func TestValidateConfig(t *testing.T) {
	t.Run("incorrect config fails", func(t *testing.T) {
		cfg := settings.NewSettings()
		cfg.Aliases = map[string][]string{
			"":                     {"value1"},
			"-bad":                 {"value2"},
			"has space":            {"value3"},
			"validalias":           {"value4"},
			"validalias-noentries": {},
		}

		errs := cfg.Validate()
		if len(errs) != 4 {
			t.Errorf("expected 4 errors, got %d", len(errs))
		}

		if len(cfg.Aliases) != 1 {
			t.Errorf("expected Aliases to have one valid entry, got %v", cfg.Aliases)
		}
	})

	t.Run("valid config passes", func(t *testing.T) {
		cfg := settings.NewSettings()

		errs := cfg.Validate()
		if errs != nil {
			t.Errorf("expected error slice to be nil, got %d errors: %v", len(errs), errs)
		}
	})
}

func TestSetConfigValue(t *testing.T) {
	t.Run("Set int field successfully", func(t *testing.T) {
		cfg := settings.NewSettings()

		err := cfg.SetValue("option.min_score", "4")
		if err != nil {
			t.Fatalf("expected option.min_score to be set, err = %v", err)
		}

		expected := int64(4)
		actual := cfg.Option.MinScore

		if expected != actual {
			t.Fatalf("expected option.min_score = %v, actual = %v", expected, actual)
		}
	})

	t.Run("Set string field successfully", func(t *testing.T) {
		cfg := settings.NewSettings()

		err := cfg.SetValue("config_location", "/home/user")
		if err != nil {
			t.Fatalf("expected config_location to be set, err = %v", err)
		}

		expected := "/home/user"
		actual := cfg.ConfigLocation

		if expected != actual {
			t.Fatalf("expected config_location = %v, actual = %v", expected, actual)
		}
	})

	t.Run("Set boolean field successfully", func(t *testing.T) {
		cfg := settings.NewSettings()

		err := cfg.SetValue("apply.imply_impure_with_tag", "true")
		if err != nil {
			t.Fatalf("expected apply.imply_impure_with_tag to be set, err = %v", err)
		}

		expected := true
		actual := cfg.Apply.ImplyImpureWithTag

		if expected != actual {
			t.Fatalf("expected apply.imply_impure_with_tag = %v, actual = %v", expected, actual)
		}
	})

	t.Run("Invalid key", func(t *testing.T) {
		cfg := settings.NewSettings()

		err := cfg.SetValue("invalid_key", "")
		if err == nil {
			t.Fatalf("expected invalid_key to error out, no errors detected")
		}
	})

	t.Run("Invalid nested key", func(t *testing.T) {
		cfg := settings.NewSettings()

		err := cfg.SetValue("apply.invalid.nested", "")
		if err == nil {
			t.Fatalf("expected apply.invalid.nested to error out, no errors detected")
		}
	})

	t.Run("Invalid boolean value", func(t *testing.T) {
		cfg := settings.NewSettings()

		err := cfg.SetValue("apply.imply_impure_with_tag", "invalid")
		if err == nil {
			t.Fatalf("expected apply.imply_impure_with_tag to error out, no errors detected")
		}
	})

	t.Run("Invalid int value", func(t *testing.T) {
		cfg := settings.NewSettings()

		err := cfg.SetValue("option.min_score", "invalid")
		if err == nil {
			t.Fatalf("expected option.min_score to error out, no errors detected")
		}

		expected := int64(1)
		actual := cfg.Option.MinScore

		if expected != actual {
			t.Fatalf("expected option.min_score to remain unchanged, expected = %v actual = %v", expected, actual)
		}
	})
}
