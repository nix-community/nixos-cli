package settings

import (
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

type Settings struct {
	Aliases        map[string][]string `koanf:"aliases" noset:"true"`
	Apply          ApplySettings       `koanf:"apply"`
	AutoRollback   bool                `koanf:"auto_rollback"`
	UseColor       bool                `koanf:"color"`
	ConfigLocation string              `koanf:"config_location"`
	Enter          EnterSettings       `koanf:"enter"`
	Init           InitSettings        `koanf:"init"`
	NoConfirm      bool                `koanf:"no_confirm"`
	Option         OptionSettings      `koanf:"option"`
	RootCommand    string              `koanf:"root_command"`
	UseNvd         bool                `koanf:"use_nvd"`
}

type ApplySettings struct {
	ImplyImpureWithTag    bool   `koanf:"imply_impure_with_tag"`
	DefaultSpecialisation string `koanf:"specialisation"`
	UseNom                bool   `koanf:"use_nom"`
	UseGitCommitMsg       bool   `koanf:"use_git_commit_msg"`
	IgnoreDirtyTree       bool   `koanf:"ignore_dirty_tree"`
}

type EnterSettings struct {
	MountResolvConf bool `koanf:"mount_resolv_conf"`
}

type InitSettings struct {
	EnableXserver bool              `koanf:"xserver_enabled"`
	DesktopConfig string            `koanf:"desktop_config"`
	ExtraAttrs    map[string]string `koanf:"extra_attrs" noset:"true"`
	ExtraConfig   string            `koanf:"extra_config" noset:"true"`
}

type OptionSettings struct {
	MinScore     int64 `koanf:"min_score"`
	Prettify     bool  `koanf:"prettify"`
	DebounceTime int64 `koanf:"debounce_time"`
}

type DescriptionEntry struct {
	Short string
	Long  string
}

const (
	aliasExample = "```\n" + `[aliases]
genlist = ["generation", "list"]
switch = ["generation", "switch"]
rollback = ["generation", "rollback"]
` + "```\n"
)

var SettingsDocs = map[string]DescriptionEntry{
	"aliases": {
		Short: "Shortcuts for long commands",
		Long:  "Defines alternative aliases for long commands to improve user ergonomics.\nExample:\n" + aliasExample,
	},
	"apply": {
		Short: "Settings for `apply` command",
	},
	"apply.imply_impure_with_tag": {
		Short: "Add --impure automatically when using --tag with flakes",
		Long:  "Automatically appends '--impure' to the 'apply' command when using '--tag' in flake-based workflows.",
	},
	"apply.specialisation": {
		Short: "Name of specialisation to use by default when activating",
		Long:  "Specifies which systemd specialisation to use when activating a configuration with 'apply'.",
	},
	"apply.use_nom": {
		Short: "Use 'nix-output-monitor' as an alternative 'nix build' frontend",
		Long:  "Enables nix-output-monitor to show more user-friendly build progress output for the 'apply' command.",
	},
	"apply.use_git_commit_msg": {
		Short: "Use last git commit message for --tag by default",
		Long:  "When enabled, the last Git commit message will be used as the value for '--tag' automatically.",
	},
	"apply.ignore_dirty_tree": {
		Short: "Ignore dirty working tree when using Git commit message for --tag",
		Long:  "Allows 'apply' to use Git commit messages even when the working directory is dirty.",
	},
	"auto_rollback": {
		Short: "Automatically rollback profile on activation failure",
		Long: "Enables automatic rollback of a NixOS system profile when an activation command fails. This can be " +
			"disabled when a reboot or some other circumstance is needed for successful activation",
	},
	"color": {
		Short: "Enable colored output",
		Long:  "Turns on ANSI color sequences for decorated output in supported terminals.",
	},
	"config_location": {
		Short: "Where to look for configuration by default",
		Long:  "Path to a Nix file or directory to look for user configuration in by default.",
	},
	"enter": {
		Short: "Settings for `enter` command",
	},
	"enter.mount_resolv_conf": {
		Short: "Bind-mount host 'resolv.conf' inside chroot for internet accesss",
		Long:  "Ensures internet access by mounting the host's /etc/resolv.conf into the chroot environment.",
	},
	"init": {
		Short: "Settings for `init` command",
	},
	"init.xserver_enabled": {
		Short: "Generate options to enable X11 display server",
		Long:  "Controls whether X11-related services and packages are configured by default during init.",
	},
	"init.desktop_config": {
		Short: "Config options for desktop environment",
		Long:  "Specifies the desktop environment configuration to inject during initialization.",
	},
	"no_confirm": {
		Short: "Disable interactive confirmation input",
		Long:  "Disables prompts that ask for user confirmation, useful for automation.",
	},
	"option": {
		Short: "Settings for `option` command",
	},
	"option.min_score": {
		Short: "Minimum distance score to consider an option a match",
		Long:  "Sets the cutoff score for showing results in fuzzy-matched option lookups.",
	},
	"option.prettify": {
		Short: "Attempt to render options using Markdown",
		Long:  "If enabled, renders option documentation in a prettier Markdown format where applicable.",
	},
	"option.debounce_time": {
		Short: "Debounce time for searching options using the UI, in milliseconds",
		Long:  "Controls how often search results are recomputed when typing in the options UI, in milliseconds.",
	},
	"root_command": {
		Short: "Command to use to promote process to root",
		Long:  "Specifies which command to use for privilege escalation (e.g., sudo or doas).",
	},
	"use_nvd": {
		Short: "Use 'nvd' instead of `nix store diff-closures`",
		Long:  "Use the better-looking `nvd` diffing tool when comparing configurations instead of `nix store diff-closures`.",
	},
}

func NewSettings() *Settings {
	return &Settings{
		AutoRollback:   true,
		UseColor:       true,
		ConfigLocation: "/etc/nixos",
		Enter: EnterSettings{
			MountResolvConf: true,
		},
		Init:        InitSettings{},
		RootCommand: "sudo",
		Option: OptionSettings{
			MinScore:     1,
			Prettify:     true,
			DebounceTime: 25,
		},
	}
}

func ParseSettings(location string) (*Settings, error) {
	k := koanf.New(".")

	if err := k.Load(file.Provider(location), toml.Parser()); err != nil {
		return nil, err
	}

	cfg := NewSettings()

	err := k.Unmarshal("", cfg)
	if err != nil {
		return nil, err
	}

	return cfg, nil
}

var hasWhitespaceRegex = regexp.MustCompile(`\s`)

// Validate the configuration and remove any erroneous values.
// A list of detected errors is returned, if any exist.
func (cfg *Settings) Validate() SettingsErrors {
	errs := []SettingsError{}

	// First, validate the aliases. Any alias has to adhere to the following rules:
	// 1. Alias names cannot be empty.
	// 2. Alias names cannot have whitespace
	// 3. Alias names cannot start with a -
	// 4. Resolved arguments list must have a len > 1
	for alias, resolved := range cfg.Aliases {
		if len(alias) == 0 {
			errs = append(errs, SettingsError{Field: "aliases", Message: "alias name cannot be empty"})
			delete(cfg.Aliases, alias)
		} else if alias[0] == '-' {
			errs = append(errs, SettingsError{Field: fmt.Sprintf("aliases.%s", alias), Message: "alias cannot start with a '-'"})
			delete(cfg.Aliases, alias)
		} else if hasWhitespaceRegex.MatchString(alias) {
			errs = append(errs, SettingsError{Field: fmt.Sprintf("aliases.%s", alias), Message: "alias cannot have whitespace"})
			delete(cfg.Aliases, alias)
		} else if len(resolved) == 0 {
			errs = append(errs, SettingsError{Field: fmt.Sprintf("aliases.%s", alias), Message: "args list cannot be empty"})
			delete(cfg.Aliases, alias)
		}
	}

	if len(errs) > 0 {
		return errs
	}
	return nil
}

func (cfg *Settings) SetValue(key string, value string) error {
	fields := strings.Split(key, ".")
	current := reflect.ValueOf(cfg).Elem()

	for i, field := range fields {
		// Find the struct field with the matching koanf tag
		found := false
		for j := 0; j < current.Type().NumField(); j++ {
			fieldInfo := current.Type().Field(j)
			if fieldInfo.Tag.Get("koanf") == field {
				current = current.Field(j)
				found = true
				break
			}
		}

		if !found {
			return SettingsError{Field: field, Message: "setting not found"}
		}

		if current.Kind() == reflect.Ptr {
			if current.IsNil() {
				current.Set(reflect.New(current.Type().Elem()))
			}
			current = current.Elem()
		}

		if i == len(fields)-1 {
			if !current.CanSet() {
				return SettingsError{Field: field, Message: "cannot change value of this setting dynamically"}
			}

			switch current.Kind() {
			case reflect.String:
				current.SetString(value)
			case reflect.Bool:
				boolVal, err := strconv.ParseBool(value)
				if err != nil {
					return SettingsError{Field: field, Message: fmt.Sprintf("invalid boolean value '%s' for field", value)}
				}
				current.SetBool(boolVal)
			case reflect.Int, reflect.Int64:
				intVal, err := strconv.ParseInt(value, 10, 64)
				if err != nil {
					return SettingsError{Field: field, Message: fmt.Sprintf("invalid integer value '%s' for field", value)}
				}
				current.SetInt(intVal)
			case reflect.Float64:
				floatVal, err := strconv.ParseFloat(value, 64)
				if err != nil {
					return SettingsError{Field: field, Message: fmt.Sprintf("invalid float value '%s' for field", value)}
				}
				current.SetFloat(floatVal)
			default:
				return SettingsError{Field: field, Message: "unsupported field type"}
			}

			return nil
		}
	}

	return nil
}

func isSettable(value *reflect.Value) bool {
	switch value.Kind() {
	case reflect.String, reflect.Bool, reflect.Int, reflect.Int64, reflect.Float64:
		return true
	}

	return false
}
