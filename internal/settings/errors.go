package settings

import "fmt"

type SettingsErrors []error

type SettingsError struct {
	Field   string
	Message string
}

func (e SettingsError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

type DeprecatedSettingError struct {
	Field       string
	Alternative string
}

func (e DeprecatedSettingError) Error() string {
	return fmt.Sprintf(`%s: %s
hint: %s`, e.Field, DeprecatedDocString, e.Alternative)
}
