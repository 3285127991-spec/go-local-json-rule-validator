// Package jsonvalidator is a lightweight, local-only JSON rule validator.
// It is NOT a full JSON Schema implementation; it supports five types
// (object, array, string, number, boolean) and a small set of constraints.
package jsonvalidator

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// Rule describes the expected shape of a JSON value. Pointer fields are used
// so that "not set" can be told apart from an explicit zero value.
type Rule struct {
	Type                 string           `json:"type"`
	Properties           map[string]*Rule `json:"properties,omitempty"`
	Required             []string         `json:"required,omitempty"`
	AdditionalProperties *bool            `json:"additionalProperties,omitempty"`
	Items                *Rule            `json:"items,omitempty"`
	MinItems             *int             `json:"minItems,omitempty"`
	MaxItems             *int             `json:"maxItems,omitempty"`
	MinLength            *int             `json:"minLength,omitempty"`
	MaxLength            *int             `json:"maxLength,omitempty"`
	Enum                 []string         `json:"enum,omitempty"`
	Minimum              *float64         `json:"minimum,omitempty"`
	Maximum              *float64         `json:"maximum,omitempty"`
}

// ruleTypes lists every supported type name.
var ruleTypes = map[string]bool{
	"object":  true,
	"array":   true,
	"string":  true,
	"number":  true,
	"boolean": true,
}

// allowedConstraints maps a type to the constraint fields it may use.
var allowedConstraints = map[string][]string{
	"object":  {"properties", "required", "additionalProperties"},
	"array":   {"items", "minItems", "maxItems"},
	"string":  {"minLength", "maxLength", "enum"},
	"number":  {"minimum", "maximum"},
	"boolean": {},
}

// RuleError reports one or more problems found in a rule definition.
type RuleError struct {
	Problems []Error
}

func (e *RuleError) Error() string {
	parts := make([]string, len(e.Problems))
	for i, p := range e.Problems {
		parts[i] = p.Path + ": " + p.Message
	}
	return "invalid rule: " + strings.Join(parts, "; ")
}

// ParseRule parses a rule from JSON text and validates it. Unknown JSON keys
// and obviously illegal configurations are rejected with a *RuleError.
func ParseRule(text string) (*Rule, error) {
	dec := json.NewDecoder(strings.NewReader(text))
	dec.DisallowUnknownFields()
	var r Rule
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("invalid rule JSON: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return nil, fmt.Errorf("invalid rule JSON: unexpected trailing data")
	}
	if err := CheckRule(&r); err != nil {
		return nil, err
	}
	return &r, nil
}

// CheckRule validates a rule definition. It returns nil for a valid rule and
// a *RuleError describing every problem otherwise.
func CheckRule(r *Rule) error {
	problems := checkRule(r, "$")
	if len(problems) > 0 {
		return &RuleError{Problems: problems}
	}
	return nil
}

func checkRule(r *Rule, path string) []Error {
	if r == nil {
		return []Error{{Path: path, Type: "rule", Message: "rule must not be null"}}
	}
	var problems []Error
	add := func(format string, args ...any) {
		problems = append(problems, Error{Path: path, Type: "rule", Message: fmt.Sprintf(format, args...)})
	}

	if r.Type == "" {
		add("type is required")
	} else if !ruleTypes[r.Type] {
		add("unknown type %q", r.Type)
	}

	if allowed, ok := allowedConstraints[r.Type]; ok {
		permits := map[string]bool{}
		for _, name := range allowed {
			permits[name] = true
		}
		for _, f := range []struct {
			name string
			set  bool
		}{
			{"properties", r.Properties != nil},
			{"required", r.Required != nil},
			{"additionalProperties", r.AdditionalProperties != nil},
			{"items", r.Items != nil},
			{"minItems", r.MinItems != nil},
			{"maxItems", r.MaxItems != nil},
			{"minLength", r.MinLength != nil},
			{"maxLength", r.MaxLength != nil},
			{"enum", r.Enum != nil},
			{"minimum", r.Minimum != nil},
			{"maximum", r.Maximum != nil},
		} {
			if f.set && !permits[f.name] {
				add("%s is not allowed for type %q", f.name, r.Type)
			}
		}
	}

	if r.Minimum != nil && r.Maximum != nil && *r.Minimum > *r.Maximum {
		add("minimum %s is greater than maximum %s", formatNum(*r.Minimum), formatNum(*r.Maximum))
	}
	if r.MinLength != nil && *r.MinLength < 0 {
		add("minLength must not be negative")
	}
	if r.MaxLength != nil && *r.MaxLength < 0 {
		add("maxLength must not be negative")
	}
	if r.MinLength != nil && r.MaxLength != nil && *r.MinLength > *r.MaxLength {
		add("minLength %d is greater than maxLength %d", *r.MinLength, *r.MaxLength)
	}
	if r.MinItems != nil && *r.MinItems < 0 {
		add("minItems must not be negative")
	}
	if r.MaxItems != nil && *r.MaxItems < 0 {
		add("maxItems must not be negative")
	}
	if r.MinItems != nil && r.MaxItems != nil && *r.MinItems > *r.MaxItems {
		add("minItems %d is greater than maxItems %d", *r.MinItems, *r.MaxItems)
	}
	if r.Enum != nil && len(r.Enum) == 0 {
		add("enum must not be empty")
	}
	seen := map[string]bool{}
	for _, name := range r.Required {
		if name == "" {
			add("required field name must not be empty")
			continue
		}
		if seen[name] {
			add("duplicate required field %q", name)
			continue
		}
		seen[name] = true
	}

	keys := make([]string, 0, len(r.Properties))
	for key := range r.Properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		problems = append(problems, checkRule(r.Properties[key], childPath(childPath(path, "properties"), key))...)
	}
	if r.Items != nil {
		problems = append(problems, checkRule(r.Items, childPath(path, "items"))...)
	}
	return problems
}
