package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strings"

	"github.com/nix-community/nixos-cli/internal/settings"
	"github.com/pelletier/go-toml/v2"
	"github.com/spf13/cobra"
	"snare.dev/optnix/option"
)

func main() {
	rootCmd := &cobra.Command{
		Use: "build",
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
			HiddenDefaultCmd:  true,
		},
	}

	var gitRev string

	siteCmd := &cobra.Command{
		Use:          "site",
		Short:        "Generate Markdown documentation for settings and modules",
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("generating settings documentation")

			generatedSettingsPath := filepath.Join("doc", "src", "generated-settings.md")
			if err := generateSettingsDocMarkdown(generatedSettingsPath); err != nil {
				return err
			}

			fmt.Println("generating module documentation")

			generatedModulePath := filepath.Join("doc", "src", "generated-module.md")
			if err := generateModuleDoc(generatedModulePath, gitRev); err != nil {
				return err
			}

			fmt.Println("generated settings and modules for mdbook site")

			return nil
		},
	}
	siteCmd.Flags().StringVarP(&gitRev, "revision", "r", "main", "Git rev to use when generating module doc links")

	var outputManDir string

	manCmd := &cobra.Command{
		Use:   "man",
		Short: "Generate man pages using scdoc",
		RunE: func(cmd *cobra.Command, args []string) error {
			return generateManPages(filepath.Join("doc", "man"), outputManDir)
		},
	}
	manCmd.Flags().StringVarP(&outputManDir, "output", "o", "man", "Where to place generated man pages")

	rootCmd.AddCommand(siteCmd, manCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func generateSettingsDocMarkdown(filename string) error {
	var sb strings.Builder

	defaults := *settings.NewSettings()

	writeSettingsDoc(reflect.TypeFor[settings.Settings](), reflect.ValueOf(defaults), "", &sb, 2, MarkdownSettingsFormatter{})

	return os.WriteFile(filename, []byte(sb.String()), 0o644)
}

//go:embed man/nixos-cli-settings.5.scd.template
var settingsTemplate string

func generateSettingsDocManpage(filename string) error {
	var sb strings.Builder

	defaults := *settings.NewSettings()

	writeSettingsDoc(reflect.TypeFor[settings.Settings](), reflect.ValueOf(defaults), "", &sb, 2, ManpageSettingsFormatter{})

	contents := fmt.Sprintf(settingsTemplate, sb.String())

	return os.WriteFile(filename, []byte(contents), 0o644)
}

func generateModuleDoc(filename string, rev string) error {
	fullOptionSrc, err := buildModuleOptionsJSON()
	if err != nil {
		return err
	}

	var options []option.NixosOption
	for _, o := range fullOptionSrc {
		if !strings.HasPrefix(o.Name, "services.nixos-cli") {
			continue
		}

		options = append(options, o)
	}

	var sb strings.Builder

	for _, opt := range options {
		sb.WriteString(formatOptionMarkdown(opt, rev))
		sb.WriteString("\n")
	}

	err = os.WriteFile(filename, []byte(sb.String()), 0o644)
	if err != nil {
		return err
	}

	return nil
}

func buildModuleOptionsJSON() (option.NixosOptionSource, error) {
	buildModuleDocArgv := []string{"nix-build", "./doc/options.nix"}

	var buildModuleDocStdout bytes.Buffer
	var buildModuleDocStderr bytes.Buffer

	buildModuleDocCmd := exec.Command(buildModuleDocArgv[0], buildModuleDocArgv[1:]...)
	buildModuleDocCmd.Stdout = &buildModuleDocStdout
	buildModuleDocCmd.Stderr = &buildModuleDocStderr

	err := buildModuleDocCmd.Run()
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, "failed to build docs")
		_, _ = fmt.Fprintf(os.Stderr, "build logs:\n%s\n", buildModuleDocStderr.String())
		return nil, err
	}

	optionsDocFilename := strings.TrimSpace(buildModuleDocStdout.String())

	optionsDocFile, err := os.Open(optionsDocFilename)
	if err != nil {
		return nil, err
	}
	defer func() { _ = optionsDocFile.Close() }()

	return option.LoadOptions(optionsDocFile)
}

func formatOptionMarkdown(opt option.NixosOption, rev string) string {
	var sb strings.Builder

	titleWasGenerated := false

	if len(opt.Declarations) > 0 {
		// Strip the /nix/store/<hash>-<name> from /nix/store/<hash>-<name>/path/to/module
		declPath := opt.Declarations[0]
		declPathParts := strings.Split(filepath.Clean(declPath), string(filepath.Separator))

		if len(declPathParts) > 4 {
			modulePath := filepath.Join(declPathParts[4:]...)
			optionURL := fmt.Sprintf("https://github.com/nix-community/nixos-cli/blob/%s/%s", rev, modulePath)

			fmt.Fprintf(&sb, "## [`%s`](%s)\n\n", opt.Name, optionURL)
			titleWasGenerated = true
		}
	}

	if !titleWasGenerated {
		fmt.Fprintf(&sb, "## `%s`\n\n", opt.Name)
	}

	if opt.Description != "" {
		sb.WriteString(opt.Description + "\n\n")
	}

	fmt.Fprintf(&sb, "**Type:** `%s`\n\n", opt.Type)

	if opt.Default != nil {
		fmt.Fprintf(&sb, "**Default:** `%s`\n\n", opt.Default.Text)
	}

	if opt.Example != nil {
		fmt.Fprintf(&sb, "**Example:** `%s`\n\n", opt.Example.Text)
	}

	return sb.String()
}

type SettingsFormatter interface {
	WriteHeader(sb *strings.Builder, title string, level int)
	WriteSectionDescription(sb *strings.Builder, desc string)
	WriteItem(sb *strings.Builder, key string, desc string, exampleValue any, defaultValue any, deprecated string)
}

type MarkdownSettingsFormatter struct{}

func (f MarkdownSettingsFormatter) WriteHeader(sb *strings.Builder, title string, level int) {
	fmt.Fprintf(sb, "%s %s\n\n", strings.Repeat("#", level), title)
}

func (MarkdownSettingsFormatter) WriteSectionDescription(sb *strings.Builder, desc string) {
	sb.WriteString(desc + "\n\n")
}

func indent(text string, spaces int) string {
	if spaces <= 0 || len(text) == 0 {
		return text
	}

	var b strings.Builder
	prefix := strings.Repeat(" ", spaces)

	for {
		i := strings.IndexByte(text, '\n')
		if i == -1 {
			b.WriteString(prefix)
			b.WriteString(text)
			break
		}

		b.WriteString(prefix)
		b.WriteString(text[:i+1])
		text = text[i+1:]
	}

	return b.String()
}

func (f MarkdownSettingsFormatter) WriteItem(sb *strings.Builder, key string, desc string, exampleValue any, defaultValue any, deprecated string) {
	fmt.Fprintf(sb, "- **%s**\n\n  %s\n\n", key, desc)

	if exampleValue != nil {
		exampleToml, err := toml.Marshal(exampleValue)
		if err != nil {
			panic(fmt.Sprintf("failed to marshal TOML value: %v", err))
		}
		exampleTomlStr := strings.TrimSpace(string(exampleToml))

		if strings.Contains(exampleTomlStr, "\n") {
			fmt.Fprintf(sb, "  **Example:**\n    ```toml\n%s\n    ```\n\n", indent(exampleTomlStr, 4))
		} else {
			fmt.Fprintf(sb, "  **Example:** `%s`\n\n", exampleTomlStr)
		}
	}

	if defaultValue != nil {
		defaultToml, err := toml.Marshal(defaultValue)
		if err != nil {
			panic(fmt.Sprintf("failed to marshal TOML value: %v", err))
		}
		defaultTomlStr := strings.TrimSpace(string(defaultToml))

		if strings.Contains(defaultTomlStr, "\n") {
			fmt.Fprintf(sb, "  **Default:**\n    ```toml\n%s\n    ```\n\n", indent(defaultTomlStr, 4))
		} else {
			fmt.Fprintf(sb, "  **Default:** `%s`\n\n", defaultTomlStr)
		}
	}

	if deprecated != "" {
		fmt.Fprintf(sb, "  **%s**\n\n  %s\n\n", settings.DeprecatedDocString, deprecated)
	}
}

type ManpageSettingsFormatter struct{}

func (f ManpageSettingsFormatter) WriteHeader(sb *strings.Builder, title string, level int) {
	fmt.Fprintf(sb, "\n%s %s\n", strings.Repeat("#", level), strings.ToUpper(title))
}

func (ManpageSettingsFormatter) WriteSectionDescription(sb *strings.Builder, desc string) {
	sb.WriteString(desc + "\n\n")
}

func (f ManpageSettingsFormatter) WriteItem(
	sb *strings.Builder,
	key string,
	desc string,
	exampleValue any,
	defaultValue any,
	deprecated string,
) {
	fmt.Fprintf(sb, "\n*%s*\n\n%s\n\n", key, desc)

	if exampleValue != nil {
		exampleToml, err := toml.Marshal(exampleValue)
		if err != nil {
			panic(fmt.Sprintf("failed to marshal TOML example: %v", err))
		}
		if exampleTomlStr := strings.TrimSpace(string(exampleToml)); exampleTomlStr != "" {
			sb.WriteString("Example: ")

			if strings.Contains(exampleTomlStr, "\n") {
				sb.WriteString("\n\n```\n")
				sb.WriteString(exampleTomlStr)
				sb.WriteString("\n```\n\n")
			} else {
				// Inline for scalars
				sb.WriteString(exampleTomlStr)
				sb.WriteString("\n\n")
			}
		}
	}

	if defaultValue != nil {
		defaultToml, err := toml.Marshal(defaultValue)
		if err != nil {
			panic(fmt.Sprintf("failed to marshal TOML default: %v", err))
		}
		if defaultTomlStr := strings.TrimSpace(string(defaultToml)); defaultTomlStr != "" {
			sb.WriteString("Default: ")

			if strings.Contains(defaultTomlStr, "\n") {
				sb.WriteString("\n\n```\n")
				sb.WriteString(defaultTomlStr)
				sb.WriteString("\n```\n\n")
			} else {
				// Inline for scalars
				sb.WriteString(defaultTomlStr)
				sb.WriteString("\n\n")
			}
		}
	}

	if deprecated != "" {
		fmt.Fprintf(sb, "\n*%s*\n\n%s\n", settings.DeprecatedDocString, deprecated)
	}
}

func writeSettingsDoc(
	t reflect.Type,
	v reflect.Value,
	path string,
	sb *strings.Builder,
	depth int,
	formatter SettingsFormatter,
) {
	type nestedField struct {
		field    reflect.StructField
		fieldVal reflect.Value
		fullKey  string
	}

	type configKey struct {
		key          string
		desc         string
		exampleValue any
		defaultValue any
		deprecated   string
	}

	var generalItems []configKey
	var nestedFields []nestedField

	for i := range t.NumField() {
		field := t.Field(i)
		tag := field.Tag
		koanfKey := tag.Get("koanf")
		if koanfKey == "" {
			continue
		}

		fullKey := path + koanfKey
		fieldVal := v.Field(i)

		if field.Type.Kind() == reflect.Struct {
			nestedFields = append(nestedFields, nestedField{field, fieldVal, fullKey})
		} else {
			descriptions, exists := settings.SettingsDocs[fullKey]
			if !exists {
				panic("missing description for " + fullKey)
			}

			defaultVal := fieldVal.Interface()

			desc := descriptions.Long
			if desc == "" {
				desc = descriptions.Short
			}
			generalItems = append(generalItems, configKey{fullKey, desc, descriptions.Example, defaultVal, descriptions.Deprecated})
		}
	}

	if len(generalItems) > 0 {
		if path == "" {
			formatter.WriteHeader(sb, "General", 2)
		}

		sort.Slice(generalItems, func(i, j int) bool {
			return generalItems[i].key < generalItems[j].key
		})

		for _, item := range generalItems {
			formatter.WriteItem(sb, item.key, item.desc, item.exampleValue, item.defaultValue, item.deprecated)
		}
	}

	for _, entry := range nestedFields {
		descriptions := settings.SettingsDocs[entry.fullKey]
		desc := descriptions.Long
		if desc == "" {
			desc = descriptions.Short
		}

		formatter.WriteHeader(sb, entry.fullKey, depth)
		formatter.WriteSectionDescription(sb, desc)
		writeSettingsDoc(entry.field.Type, entry.fieldVal, entry.fullKey+".", sb, depth+1, formatter)
	}
}

func generateManPages(inputDir string, outputDir string) error {
	generatedSettingsManPagePath := filepath.Join("doc", "man", "nixos-cli-settings.5.scd")

	if err := generateSettingsDocManpage(generatedSettingsManPagePath); err != nil {
		return err
	}

	return filepath.WalkDir(inputDir, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}

		if filepath.Ext(path) != ".scd" {
			return nil
		}

		content, readErr := os.ReadFile(path)
		if readErr != nil {
			return fmt.Errorf("failed to read %s: %w", path, readErr)
		}

		cmd := exec.Command("scdoc")
		cmd.Stdin = bytes.NewReader(content)

		var outBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = os.Stderr

		if err = cmd.Run(); err != nil {
			return fmt.Errorf("scdoc failed for %s: %w", path, err)
		}

		base := filepath.Base(path)
		manFile := base[:len(base)-len(".scd")]
		outPath := filepath.Join(outputDir, manFile)

		if err = os.MkdirAll(outputDir, 0o755); err != nil {
			return err
		}

		if writeErr := os.WriteFile(outPath, outBuf.Bytes(), 0o644); writeErr != nil {
			return fmt.Errorf("failed to write %s: %w", outPath, writeErr)
		}

		fmt.Printf("generated %s\n", outPath)
		return nil
	})
}
