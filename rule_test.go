package jsonvalidator_test

import (
	"errors"
	"strings"
	"testing"

	"jsonvalidator"
)

func TestParseRuleValid(t *testing.T) {
	rule, err := jsonvalidator.ParseRule(`{
	  "type": "object",
	  "properties": {
	    "name": {"type": "string", "minLength": 1, "maxLength": 20, "enum": ["a", "b"]},
	    "age": {"type": "number", "minimum": 0, "maximum": 150},
	    "tags": {"type": "array", "items": {"type": "string"}, "minItems": 1, "maxItems": 3},
	    "active": {"type": "boolean"}
	  },
	  "required": ["name"],
	  "additionalProperties": false
	}`)
	if err != nil {
		t.Fatalf("ParseRule: %v", err)
	}
	if rule.Type != "object" || len(rule.Properties) != 4 {
		t.Fatalf("unexpected rule: %+v", rule)
	}
	tags := rule.Properties["tags"]
	if tags.MinItems == nil || *tags.MinItems != 1 || tags.MaxItems == nil || *tags.MaxItems != 3 {
		t.Fatalf("minItems/maxItems not parsed: %+v", tags)
	}
	if rule.AdditionalProperties == nil || *rule.AdditionalProperties != false {
		t.Fatal("additionalProperties=false not parsed")
	}
	age := rule.Properties["age"]
	if age.Minimum == nil || *age.Minimum != 0 {
		t.Fatal("explicit minimum=0 must be preserved")
	}
}

func TestParseRuleMalformedJSON(t *testing.T) {
	if _, err := jsonvalidator.ParseRule("{"); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if _, err := jsonvalidator.ParseRule(`{"type": "string"} trailing`); err == nil {
		t.Fatal("expected error for trailing data")
	}
	if _, err := jsonvalidator.ParseRule(`{"type": "string", "minLen": 1}`); err == nil {
		t.Fatal("expected error for unknown key (misspelled minLength)")
	}
}

func TestCheckRuleInvalidConfigs(t *testing.T) {
	cases := []struct {
		name string
		rule string
		want string // substring expected in the RuleError message
	}{
		{"unknown type", `{"type": "integer"}`, `unknown type "integer"`},
		{"missing type", `{}`, "type is required"},
		{"minimum > maximum", `{"type": "number", "minimum": 5, "maximum": 1}`, "minimum 5 is greater than maximum 1"},
		{"minLength > maxLength", `{"type": "string", "minLength": 5, "maxLength": 2}`, "minLength 5 is greater than maxLength 2"},
		{"minItems > maxItems", `{"type": "array", "minItems": 3, "maxItems": 1}`, "minItems 3 is greater than maxItems 1"},
		{"negative minLength", `{"type": "string", "minLength": -1}`, "minLength must not be negative"},
		{"negative maxItems", `{"type": "array", "maxItems": -2}`, "maxItems must not be negative"},
		{"empty enum", `{"type": "string", "enum": []}`, "enum must not be empty"},
		{"constraint on wrong type", `{"type": "number", "minLength": 1}`, `minLength is not allowed for type "number"`},
		{"items on string", `{"type": "string", "items": {"type": "string"}}`, `items is not allowed for type "string"`},
		{"duplicate required", `{"type": "object", "required": ["a", "a"]}`, `duplicate required field "a"`},
		{"empty required name", `{"type": "object", "required": [""]}`, "required field name must not be empty"},
		{"null nested rule", `{"type": "object", "properties": {"a": null}}`, "rule must not be null"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := jsonvalidator.ParseRule(tc.rule)
			if err == nil {
				t.Fatalf("expected rule to be rejected")
			}
			var ruleErr *jsonvalidator.RuleError
			if !errors.As(err, &ruleErr) {
				t.Fatalf("expected *RuleError, got %T: %v", err, err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q should contain %q", err.Error(), tc.want)
			}
		})
	}
}

func TestCheckRuleNestedProblemPath(t *testing.T) {
	_, err := jsonvalidator.ParseRule(`{
	  "type": "object",
	  "properties": {
	    "user": {
	      "type": "object",
	      "properties": {"name": {"type": "str"}}
	    }
	  }
	}`)
	var ruleErr *jsonvalidator.RuleError
	if !errors.As(err, &ruleErr) {
		t.Fatalf("expected *RuleError, got %T: %v", err, err)
	}
	if len(ruleErr.Problems) != 1 {
		t.Fatalf("want 1 problem, got %v", ruleErr.Problems)
	}
	if want := "$.properties.user.properties.name"; ruleErr.Problems[0].Path != want {
		t.Fatalf("problem path = %q, want %q", ruleErr.Problems[0].Path, want)
	}
}

func TestCheckRuleMultipleProblems(t *testing.T) {
	_, err := jsonvalidator.ParseRule(`{
	  "type": "object",
	  "properties": {
	    "a": {"type": "number", "minimum": 9, "maximum": 1},
	    "b": {"type": "string", "minLength": 4, "maxLength": 2}
	  }
	}`)
	var ruleErr *jsonvalidator.RuleError
	if !errors.As(err, &ruleErr) {
		t.Fatalf("expected *RuleError, got %T: %v", err, err)
	}
	if len(ruleErr.Problems) != 2 {
		t.Fatalf("want 2 problems, got %v", ruleErr.Problems)
	}
	// Problems follow a deterministic order (sorted property names).
	if ruleErr.Problems[0].Path != "$.properties.a" || ruleErr.Problems[1].Path != "$.properties.b" {
		t.Fatalf("unexpected problem order: %v", ruleErr.Problems)
	}
}
