package jsonvalidator

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"unicode/utf8"
)

// Error describes a single validation failure. Path uses a JSONPath-like
// syntax rooted at "$", e.g. "$.user.name" or "$.items[2].price".
type Error struct {
	Path    string `json:"path"`
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (e Error) String() string {
	return fmt.Sprintf("%s: %s", e.Path, e.Message)
}

// Validation error kinds, used in Error.Type.
const (
	ErrType         = "type"
	ErrRequired     = "required"
	ErrUnknownField = "unknown_field"
	ErrMinLength    = "min_length"
	ErrMaxLength    = "max_length"
	ErrEnum         = "enum"
	ErrMinimum      = "minimum"
	ErrMaximum      = "maximum"
	ErrMinItems     = "min_items"
	ErrMaxItems     = "max_items"
)

// Validator validates JSON data against a fixed rule. A Validator is
// immutable after creation and safe for concurrent use by multiple
// goroutines.
type Validator struct {
	rule *Rule
}

// New creates a Validator for the given rule. The rule is checked first and
// an invalid rule yields a *RuleError. The caller must not mutate the rule
// afterwards.
func New(rule *Rule) (*Validator, error) {
	if err := CheckRule(rule); err != nil {
		return nil, err
	}
	return &Validator{rule: rule}, nil
}

// Validate checks data against the rule and returns every error found,
// sorted by path, then by error type, then by message, so the output order
// is stable across calls. It returns nil when data is valid. Data is
// expected to come from ParseData or encoding/json unmarshalling into any.
func (v *Validator) Validate(data any) []Error {
	var errs []Error
	v.validateValue(v.rule, data, "$", &errs)
	if len(errs) == 0 {
		return nil
	}
	sort.SliceStable(errs, func(i, j int) bool {
		if errs[i].Path != errs[j].Path {
			return errs[i].Path < errs[j].Path
		}
		if errs[i].Type != errs[j].Type {
			return errs[i].Type < errs[j].Type
		}
		return errs[i].Message < errs[j].Message
	})
	return errs
}

func (v *Validator) validateValue(rule *Rule, data any, path string, errs *[]Error) {
	add := func(kind, format string, args ...any) {
		*errs = append(*errs, Error{Path: path, Type: kind, Message: fmt.Sprintf(format, args...)})
	}
	switch rule.Type {
	case "object":
		obj, ok := data.(map[string]any)
		if !ok {
			add(ErrType, "expected object, got %s", typeName(data))
			return
		}
		for _, name := range rule.Required {
			if _, ok := obj[name]; !ok {
				*errs = append(*errs, Error{
					Path:    childPath(path, name),
					Type:    ErrRequired,
					Message: fmt.Sprintf("required field %q is missing", name),
				})
			}
		}
		strict := rule.AdditionalProperties != nil && !*rule.AdditionalProperties
		for key, val := range obj {
			sub, known := rule.Properties[key]
			if !known {
				if strict {
					*errs = append(*errs, Error{
						Path:    childPath(path, key),
						Type:    ErrUnknownField,
						Message: fmt.Sprintf("unknown field %q", key),
					})
				}
				continue
			}
			v.validateValue(sub, val, childPath(path, key), errs)
		}
	case "array":
		arr, ok := data.([]any)
		if !ok {
			add(ErrType, "expected array, got %s", typeName(data))
			return
		}
		if rule.MinItems != nil && len(arr) < *rule.MinItems {
			add(ErrMinItems, "array has %d item(s), fewer than minItems %d", len(arr), *rule.MinItems)
		}
		if rule.MaxItems != nil && len(arr) > *rule.MaxItems {
			add(ErrMaxItems, "array has %d item(s), more than maxItems %d", len(arr), *rule.MaxItems)
		}
		if rule.Items != nil {
			for i, item := range arr {
				v.validateValue(rule.Items, item, indexPath(path, i), errs)
			}
		}
	case "string":
		s, ok := data.(string)
		if !ok {
			add(ErrType, "expected string, got %s", typeName(data))
			return
		}
		n := utf8.RuneCountInString(s)
		if rule.MinLength != nil && n < *rule.MinLength {
			add(ErrMinLength, "string length %d is less than minLength %d", n, *rule.MinLength)
		}
		if rule.MaxLength != nil && n > *rule.MaxLength {
			add(ErrMaxLength, "string length %d is greater than maxLength %d", n, *rule.MaxLength)
		}
		if rule.Enum != nil {
			found := false
			for _, allowed := range rule.Enum {
				if s == allowed {
					found = true
					break
				}
			}
			if !found {
				enumJSON, _ := json.Marshal(rule.Enum)
				add(ErrEnum, "value %q is not one of %s", s, enumJSON)
			}
		}
	case "number":
		num, ok := data.(float64)
		if !ok {
			add(ErrType, "expected number, got %s", typeName(data))
			return
		}
		if rule.Minimum != nil && num < *rule.Minimum {
			add(ErrMinimum, "value %s is less than minimum %s", formatNum(num), formatNum(*rule.Minimum))
		}
		if rule.Maximum != nil && num > *rule.Maximum {
			add(ErrMaximum, "value %s is greater than maximum %s", formatNum(num), formatNum(*rule.Maximum))
		}
	case "boolean":
		if _, ok := data.(bool); !ok {
			add(ErrType, "expected boolean, got %s", typeName(data))
		}
	}
}

var identRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

// childPath appends an object key to a JSONPath-like path, using dot
// notation for identifier-like keys and bracket notation otherwise.
func childPath(parent, key string) string {
	if identRe.MatchString(key) {
		return parent + "." + key
	}
	enc, _ := json.Marshal(key)
	return parent + "[" + string(enc) + "]"
}

// indexPath appends an array index to a JSONPath-like path.
func indexPath(parent string, i int) string {
	return parent + "[" + strconv.Itoa(i) + "]"
}

// typeName names the JSON type of a decoded value for error messages.
func typeName(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return fmt.Sprintf("%T", v)
	}
}

func formatNum(f float64) string {
	return strconv.FormatFloat(f, 'g', -1, 64)
}
