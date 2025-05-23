package option

import (
	"encoding/json"
	"os"
)

type NixosOption struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	Type         string            `json:"type"`
	Default      *NixosOptionValue `json:"default"`
	Example      *NixosOptionValue `json:"example"`
	Location     []string          `json:"loc"`
	ReadOnly     bool              `json:"readOnly"`
	Declarations []string          `json:"declarations"`
}

type NixosOptionValue struct {
	Type string `json:"_type"`
	Text string `json:"text"`
}

type NixosOptionSource []NixosOption

func (o NixosOptionSource) String(i int) string {
	return o[i].Name
}

func (o NixosOptionSource) Len() int {
	return len(o)
}

func LoadOptionsFromFile(path string) (NixosOptionSource, error) {
	f, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var options []NixosOption
	err = json.Unmarshal(f, &options)
	if err != nil {
		return nil, err
	}

	return options, nil
}
