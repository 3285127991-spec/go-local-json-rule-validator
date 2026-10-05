package jsonvalidator_test

import (
	"strings"
	"testing"

	"jsonvalidator"
)

// TestInvalidRules 非法规则必须在解析或创建 Validator 时被拒绝。
func TestInvalidRules(t *testing.T) {
	cases := []struct {
		name    string
		rule    string
		wantErr string
	}{
		{"unknown type", `{"type": "integer"}`, "unknown type"},
		{"missing type", `{}`, "missing"},
		{"minimum gt maximum", `{"type": "number", "minimum": 5, "maximum": 1}`, "minimum"},
		{"minLength gt maxLength", `{"type": "string", "minLength": 5, "maxLength": 2}`, "minLength"},
		{"minItems gt maxItems", `{"type": "array", "minItems": 3, "maxItems": 1}`, "minItems"},
		{"negative minLength", `{"type": "string", "minLength": -1}`, "negative"},
		{"negative maxItems", `{"type": "array", "maxItems": -2}`, "negative"},
		{"unknown rule key", `{"type": "object", "properites": {}}`, "unknown field"},
		{"misplaced constraint", `{"type": "string", "minimum": 1}`, "unknown field"},
		{"boolean with constraint", `{"type": "boolean", "enum": ["x"]}`, "unknown field"},
		{"empty required name", `{"type": "object", "required": [""]}`, "empty field name"},
		{"nested invalid rule", `{"type": "object", "properties": {"a": {"type": "number", "minimum": 2, "maximum": 1}}}`, `property "a"`},
		{"invalid items rule", `{"type": "array", "items": {"type": "nope"}}`, "unknown type"},
		{"malformed json", `{"type":`, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := jsonvalidator.NewValidatorFromJSON([]byte(tc.rule))
			if err == nil {
				t.Fatalf("expected error for rule %s", tc.rule)
			}
			if tc.wantErr != "" && !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// TestNewValidatorRejectsInvalidProgrammaticRule 以编程方式构造的
// 非法规则同样必须被 NewValidator 拒绝。
func TestNewValidatorRejectsInvalidProgrammaticRule(t *testing.T) {
	floatPtr := func(f float64) *float64 { return &f }
	intPtr := func(i int) *int { return &i }

	cases := []struct {
		name string
		rule *jsonvalidator.Rule
	}{
		{"nil rule", nil},
		{"unknown type", &jsonvalidator.Rule{Type: "weird"}},
		{"minimum gt maximum", &jsonvalidator.Rule{Type: jsonvalidator.TypeNumber, Minimum: floatPtr(5), Maximum: floatPtr(1)}},
		{"minItems gt maxItems", &jsonvalidator.Rule{Type: jsonvalidator.TypeArray, MinItems: intPtr(3), MaxItems: intPtr(1)}},
		{"constraint of other type", &jsonvalidator.Rule{Type: jsonvalidator.TypeString, Minimum: floatPtr(1)}},
		{"invalid nested rule", &jsonvalidator.Rule{
			Type:       jsonvalidator.TypeObject,
			Properties: map[string]*jsonvalidator.Rule{"a": {Type: "weird"}},
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := jsonvalidator.NewValidator(tc.rule); err == nil {
				t.Fatal("expected error")
			}
		})
	}
}

// TestValidRuleAccepted 一份覆盖全部约束的合法规则必须能解析并创建 Validator。
func TestValidRuleAccepted(t *testing.T) {
	v, err := jsonvalidator.NewValidatorFromJSON([]byte(complexRuleJSON))
	if err != nil {
		t.Fatalf("NewValidatorFromJSON failed: %v", err)
	}
	if errs := v.Validate(mustData(t, validDataJSON)); len(errs) != 0 {
		t.Fatalf("valid data got errors: %v", errs)
	}
}

// TestProgrammaticRuleWorks 以编程方式构造的合法规则可以正常校验。
func TestProgrammaticRuleWorks(t *testing.T) {
	minLen := 1
	v, err := jsonvalidator.NewValidator(&jsonvalidator.Rule{
		Type:      jsonvalidator.TypeString,
		MinLength: &minLen,
	})
	if err != nil {
		t.Fatalf("NewValidator failed: %v", err)
	}
	if errs := v.Validate(""); len(errs) != 1 || errs[0].Kind != jsonvalidator.ErrMinLength {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if errs := v.Validate("ok"); len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
}
