package nixopts

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/spf13/pflag"
)

var availableOptions = map[string]string{
	"Quiet":            string(NixOptionQuiet),
	"PrintBuildLogs":   string(NixOptionPrintBuildLogs),
	"NoBuildOutput":    string(NixOptionNoBuildOutput),
	"ShowTrace":        string(NixOptionShowTrace),
	"KeepGoing":        string(NixOptionKeepGoing),
	"KeepFailed":       string(NixOptionKeepFailed),
	"Fallback":         string(NixOptionFallback),
	"Refresh":          string(NixOptionRefresh),
	"Repair":           string(NixOptionRepair),
	"Impure":           string(NixOptionImpure),
	"Offline":          string(NixOptionOffline),
	"NoNet":            string(NixOptionNoNet),
	"MaxJobs":          string(NixOptionMaxJobs),
	"Cores":            string(NixOptionCores),
	"LogFormat":        string(NixOptionLogFormat),
	"Options":          string(NixOptionOption),
	"Builders":         string(NixOptionBuilders),
	"RecreateLockFile": string(NixOptionRecreateLockFile),
	"NoUpdateLockFile": string(NixOptionNoUpdateLockFile),
	"NoWriteLockFile":  string(NixOptionNoWriteLockFile),
	"NoUseRegistries":  string(NixOptionNoUseRegistries),
	"CommitLockFile":   string(NixOptionCommitLockFile),
	"UpdateInputs":     string(NixOptionUpdateInput),
	"OverrideInputs":   string(NixOptionOverrideInput),
	"Includes":         string(NixOptionInclude),

	// This can be for both `--use-substitutes` and
	// `--substitute-on-destination`, so use the common
	// short flag alias between them for this.
	"SubstituteOnDestination": "-s",
}

func getNixFlag(name string) string {
	if option, ok := availableOptions[name]; ok {
		return option
	}

	panic("unknown option '" + name + "' when trying to convert to nix options struct")
}

func NixOptionsToArgsList(flags *pflag.FlagSet, options any) []string {
	val := reflect.ValueOf(options)
	typ := reflect.TypeOf(options)

	if val.Kind() == reflect.Ptr {
		val = val.Elem()
		typ = typ.Elem()
	}

	args := make([]string, 0)

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		nixOption := getNixFlag(fieldType.Name)

		if !flags.Changed(nixOption) {
			continue
		}

		// If the argument starts with a dash, then treat
		// it verbatim as a flag, since this probably represents
		// a special situation.
		var optionArg string
		if strings.HasPrefix(nixOption, "-") {
			optionArg = nixOption
		} else {
			optionArg = fmt.Sprintf("--%s", nixOption)
		}

		args = append(args, expandNixOptionArg(field, fieldType.Name, optionArg)...)
	}

	return args
}

func NixOptionsToArgsListByCategory(flags *pflag.FlagSet, options any, category string) []string {
	val := reflect.ValueOf(options)
	typ := reflect.TypeOf(options)

	if val.Kind() == reflect.Pointer {
		val = val.Elem()
		typ = typ.Elem()
	}

	args := make([]string, 0)

	for i := 0; i < val.NumField(); i++ {
		field := val.Field(i)
		fieldType := typ.Field(i)

		nixOption := getNixFlag(fieldType.Name)

		categories := strings.Split(fieldType.Tag.Get("nixCategory"), ",")
		if categoryIndex := slices.Index(categories, category); categoryIndex == -1 {
			continue
		}

		// Do not check if special-cased flags have changed in the
		// cobra options, since they can't be addressed properly.
		// Just add them verbatim if not zero-valued.
		var optionArg string
		if !strings.HasPrefix(nixOption, "-") {
			if !flags.Changed(nixOption) {
				continue
			}

			optionArg = fmt.Sprintf("--%s", nixOption)
		} else {
			optionArg = nixOption
		}

		args = append(args, expandNixOptionArg(field, fieldType.Name, optionArg)...)

	}

	return args
}

func expandNixOptionArg(field reflect.Value, fieldName string, optionArg string) []string {
	switch field.Kind() {
	case reflect.Bool:
		if field.Bool() {
			return []string{optionArg}
		}
	case reflect.Int:
		return []string{optionArg, fmt.Sprintf("%d", field.Int())}
	case reflect.String:
		if field.String() != "" {
			return []string{optionArg, field.String()}
		}
	case reflect.Slice:
		var result []string
		if field.Len() > 0 {
			for j := 0; j < field.Len(); j++ {
				result = append(result, optionArg, field.Index(j).String())
			}
		}
		return result
	case reflect.Map:
		keys := field.MapKeys()

		sort.Slice(keys, func(i, j int) bool {
			return keys[i].String() < keys[j].String()
		})

		var args []string
		for _, key := range keys {
			value := field.MapIndex(key)
			args = append(args, optionArg, key.String(), value.String())
		}
		return args
	default:
		panic("unsupported field type " + field.Kind().String() + " for field '" + fieldName + "'")
	}

	return nil
}
