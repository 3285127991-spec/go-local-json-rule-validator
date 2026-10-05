package jsonvalidator

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// ParseData parses JSON text into a generic value (map[string]any, []any,
// string, float64, bool or nil) suitable for Validator.Validate.
func ParseData(text string) (any, error) {
	dec := json.NewDecoder(strings.NewReader(text))
	var data any
	if err := dec.Decode(&data); err != nil {
		return nil, fmt.Errorf("invalid JSON data: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("invalid JSON data: unexpected trailing data")
	}
	return data, nil
}
