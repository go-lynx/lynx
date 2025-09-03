package plugin

import (
	"encoding/json"
	"io"

	"gopkg.in/yaml.v3"
)

// exportJSON exports data as JSON
func exportJSON(w io.Writer, data interface{}) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(data)
}

// exportYAML exports data as YAML
func exportYAML(w io.Writer, data interface{}) error {
	encoder := yaml.NewEncoder(w)
	defer encoder.Close()
	return encoder.Encode(data)
}