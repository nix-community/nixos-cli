package settings

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/spf13/cobra"
)

type fieldCompleteResult struct {
	Name        string
	Description string
}

func findFieldCompletions(value any, prefix string) ([]fieldCompleteResult, bool) {
	var candidates []fieldCompleteResult

	fieldNames := strings.Split(prefix, ".")

	finalFieldComponent := ""
	previousComponents := []string{}

	if len(fieldNames) > 0 {
		finalFieldComponent = fieldNames[len(fieldNames)-1]
		previousComponents = fieldNames[:len(fieldNames)-1]
	}

	current := reflect.ValueOf(value)
	if current.Kind() == reflect.Ptr {
		current = current.Elem()
	}

	// Traverse into the structure following all components except the final one
	for _, fieldName := range previousComponents {
		found := false

		for i := 0; i < current.Type().NumField(); i++ {
			field := current.Type().Field(i)
			if field.Tag.Get("koanf") == fieldName {
				current = current.Field(i)
				if current.Kind() == reflect.Ptr {
					if current.IsNil() {
						current.Set(reflect.New(current.Type().Elem()))
					}

					current = current.Elem()
				}

				found = true
				break
			}
		}

		if !found || current.Kind() != reflect.Struct {
			return nil, false
		}
	}

	if current.Kind() == reflect.Ptr {
		if current.IsNil() {
			current.Set(reflect.New(current.Type().Elem()))
		}
		current = current.Elem()
	}

	if current.Kind() != reflect.Struct {
		return nil, false
	}

	for i := 0; i < current.Type().NumField(); i++ {
		structField := current.Type().Field(i)

		if structField.Tag.Get("noset") == "true" {
			continue
		}

		name := structField.Tag.Get("koanf")
		if name == "" {
			continue
		}

		fullName := strings.Join(append(previousComponents, name), ".")
		description := bestDescriptionFor(fullName)

		if name == finalFieldComponent {
			field := current.Field(i)
			isComplete := isSettable(&field)
			return []fieldCompleteResult{
				{
					Name:        fullName,
					Description: description,
				},
			}, isComplete
		}

		if strings.HasPrefix(name, finalFieldComponent) {
			candidates = append(candidates, fieldCompleteResult{
				Name:        fullName,
				Description: description,
			})
		}
	}

	isComplete := false

	if len(candidates) == 1 {
		candidate := candidates[0].Name
		lastDot := strings.LastIndex(candidate, ".")
		fieldName := candidate
		if lastDot != -1 {
			fieldName = candidate[lastDot+1:]
		}

		for i := 0; i < current.Type().NumField(); i++ {
			structField := current.Type().Field(i)
			if structField.Tag.Get("koanf") == fieldName {
				field := current.Field(i)
				isComplete = isSettable(&field)
				break
			}
		}
	}

	return candidates, isComplete
}

func bestDescriptionFor(name string) string {
	best := ""
	shortestLen := -1

	for k, v := range SettingsDocs {
		if strings.HasSuffix(k, name) {
			if shortestLen == -1 || len(k) < shortestLen {
				best = v.Short
				shortestLen = len(k)
			}
		}
	}

	return best
}

func CompleteConfigFlag(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	splitIndex := strings.Index(toComplete, "=")
	if splitIndex == -1 {
		return completeKeys(toComplete)
	}

	key := toComplete[0:splitIndex]
	candidate := toComplete[splitIndex+1:]

	return completeValues(key, candidate)
}

func completeKeys(candidate string) ([]string, cobra.ShellCompDirective) {
	completionCandidates, complete := findFieldCompletions(NewSettings(), candidate)

	// There are three cases of completions where extra actions need to be taken:
	// 1. Multiple candidates remaining
	//    - Do nothing
	// 2. Single candidate, but does not represent a settable key (aka a struct)
	//    - Add a '.', more input is needed
	// 3. Single candidate, and complete key is found
	//    - Add a '=' to signify start of value completions, if they exist
	if len(completionCandidates) == 1 {
		if complete {
			completionCandidates[0].Name = completionCandidates[0].Name + "="
		} else {
			completionCandidates[0].Name = completionCandidates[0].Name + "."
		}
	}

	candidates := make([]string, len(completionCandidates))
	for i, v := range completionCandidates {
		if v.Description != "" {
			candidates[i] = fmt.Sprintf("%v\t%v", v.Name, v.Description)
		} else {
			candidates[i] = v.Name
		}
	}

	// Completion of keys should never end with a space, since the value
	// is required.
	return candidates, cobra.ShellCompDirectiveNoSpace
}

type CompletionValueFunc func(key string, candidate string) ([]string, cobra.ShellCompDirective)

func boolCompletionFunc(key string, candidate string) ([]string, cobra.ShellCompDirective) {
	return stringCompletionFunc(key, candidate, map[string]string{
		"true":  "Turn this setting on",
		"false": "Turn this setting off",
	})
}

func stringCompletionFunc(key string, candidate string, candidates map[string]string) ([]string, cobra.ShellCompDirective) {
	var matches []string

	for c, desc := range candidates {
		if strings.HasPrefix(c, candidate) {
			match := fmt.Sprintf("%v=%v\t%v", key, c, desc)
			matches = append(matches, match)
		}
	}

	return matches, cobra.ShellCompDirectiveNoFileComp
}

func completeConfirmationBehaviorKey(key, candidate string) ([]string, cobra.ShellCompDirective) {
	return stringCompletionFunc(key, candidate, AvailableConfirmationPromptSettings)
}

// For custom completion functions, use this.
var completionValueFuncs = map[string]CompletionValueFunc{
	"confirmation.empty":   completeConfirmationBehaviorKey,
	"confirmation.invalid": completeConfirmationBehaviorKey,
}

func completeValues(key string, value string) ([]string, cobra.ShellCompDirective) {
	cfg := NewSettings()

	if completeFunc, ok := completionValueFuncs[key]; ok {
		return completeFunc(key, value)
	}

	if isBoolField(cfg, key) {
		return boolCompletionFunc(key, value)
	}

	return []string{}, cobra.ShellCompDirectiveNoFileComp
}

func isBoolField(root any, key string) bool {
	field := findField(root, key)
	kind := field.Kind()
	return kind == reflect.Bool
}

func findField(root any, key string) *reflect.Value {
	parts := strings.Split(key, ".")
	current := reflect.ValueOf(root)

	if current.Kind() == reflect.Ptr {
		current = current.Elem()
	}

	for _, part := range parts {
		if current.Kind() != reflect.Struct {
			return nil
		}

		found := false
		for i := 0; i < current.Type().NumField(); i++ {
			field := current.Type().Field(i)
			if field.Tag.Get("koanf") == part {
				current = current.Field(i)
				if current.Kind() == reflect.Ptr {
					if current.IsNil() {
						return nil
					}
					current = current.Elem()
				}
				found = true
				break
			}
		}
		if !found {
			return nil
		}
	}

	return &current
}
