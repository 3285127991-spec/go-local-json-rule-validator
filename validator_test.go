package jsonvalidator_test

import (
	"reflect"
	"sync"
	"testing"

	"jsonvalidator"
)

// complexRuleJSON 覆盖五种类型、嵌套对象、数组、required、
// additionalProperties=false、enum、长度与数值约束。
const complexRuleJSON = `{
  "type": "object",
  "properties": {
    "user": {
      "type": "object",
      "properties": {
        "name": {"type": "string", "minLength": 1, "maxLength": 20},
        "age":  {"type": "number", "minimum": 0, "maximum": 150},
        "role": {"type": "string", "enum": ["admin", "user", "guest"]}
      },
      "required": ["name"],
      "additionalProperties": false
    },
    "items": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "price":  {"type": "number", "minimum": 0},
          "gift":   {"type": "boolean"}
        },
        "required": ["price"],
        "additionalProperties": false
      },
      "minItems": 1,
      "maxItems": 5
    }
  },
  "required": ["user", "items"],
  "additionalProperties": false
}`

const validDataJSON = `{
  "user": {"name": "alice", "age": 30, "role": "admin"},
  "items": [{"price": 5}, {"price": 0, "gift": true}]
}`

// badDataJSON 含有 6 处错误，分布在嵌套对象、数组元素和根对象上。
const badDataJSON = `{
  "user": {"age": -3, "extra": 1},
  "items": [{"price": 5}, {}, {"price": -2}],
  "unknown": true
}`

func mustValidator(t *testing.T, ruleJSON string) *jsonvalidator.Validator {
	t.Helper()
	v, err := jsonvalidator.NewValidatorFromJSON([]byte(ruleJSON))
	if err != nil {
		t.Fatalf("NewValidatorFromJSON failed: %v", err)
	}
	return v
}

func mustData(t *testing.T, dataJSON string) any {
	t.Helper()
	data, err := jsonvalidator.ParseDataJSON([]byte(dataJSON))
	if err != nil {
		t.Fatalf("ParseDataJSON failed: %v", err)
	}
	return data
}

// summaries 把错误列表转成 "kind path" 字符串，便于精确比较。
func summaries(errs []jsonvalidator.ValidationError) []string {
	if len(errs) == 0 {
		return nil
	}
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = string(e.Kind) + " " + e.Path
	}
	return out
}

func TestValidateFiveTypes(t *testing.T) {
	cases := []struct {
		name     string
		rule     string
		goodData string
		badData  string
	}{
		{"object", `{"type": "object"}`, `{"a": 1}`, `[1]`},
		{"array", `{"type": "array"}`, `[1, 2]`, `{"a": 1}`},
		{"string", `{"type": "string"}`, `"hi"`, `1`},
		{"number", `{"type": "number"}`, `2.5`, `"1"`},
		{"boolean", `{"type": "boolean"}`, `true`, `0`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := mustValidator(t, tc.rule)
			if errs := v.Validate(mustData(t, tc.goodData)); len(errs) != 0 {
				t.Fatalf("valid data got errors: %v", errs)
			}
			errs := v.Validate(mustData(t, tc.badData))
			want := []string{"type $"}
			if got := summaries(errs); !reflect.DeepEqual(got, want) {
				t.Fatalf("type mismatch: got %v, want %v", got, want)
			}
		})
	}
}

func TestValidateBoundaries(t *testing.T) {
	cases := []struct {
		name string
		rule string
		data string
		want []string // 期望的 "kind path" 列表；nil 表示无错误
	}{
		// string 长度边界（按 Unicode 字符数计）
		{"string min ok", `{"type":"string","minLength":2,"maxLength":4}`, `"ab"`, nil},
		{"string max ok", `{"type":"string","minLength":2,"maxLength":4}`, `"abcd"`, nil},
		{"string too short", `{"type":"string","minLength":2,"maxLength":4}`, `"a"`, []string{"min_length $"}},
		{"string too long", `{"type":"string","minLength":2,"maxLength":4}`, `"abcde"`, []string{"max_length $"}},
		{"string runes ok", `{"type":"string","minLength":2,"maxLength":4}`, `"你好"`, nil},
		{"string runes short", `{"type":"string","minLength":2,"maxLength":4}`, `"你"`, []string{"min_length $"}},
		// number 边界（含端点）
		{"number min edge", `{"type":"number","minimum":1.5,"maximum":3.5}`, `1.5`, nil},
		{"number max edge", `{"type":"number","minimum":1.5,"maximum":3.5}`, `3.5`, nil},
		{"number below min", `{"type":"number","minimum":1.5,"maximum":3.5}`, `1.49`, []string{"minimum $"}},
		{"number above max", `{"type":"number","minimum":1.5,"maximum":3.5}`, `3.51`, []string{"maximum $"}},
		// array 元素个数边界
		{"array min edge", `{"type":"array","minItems":1,"maxItems":2}`, `[1]`, nil},
		{"array max edge", `{"type":"array","minItems":1,"maxItems":2}`, `[1,2]`, nil},
		{"array too few", `{"type":"array","minItems":1,"maxItems":2}`, `[]`, []string{"min_items $"}},
		{"array too many", `{"type":"array","minItems":1,"maxItems":2}`, `[1,2,3]`, []string{"max_items $"}},
		// enum
		{"enum hit", `{"type":"string","enum":["red","green"]}`, `"red"`, nil},
		{"enum miss", `{"type":"string","enum":["red","green"]}`, `"blue"`, []string{"enum $"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := mustValidator(t, tc.rule)
			got := summaries(v.Validate(mustData(t, tc.data)))
			if !reflect.DeepEqual(got, tc.want) && !(got == nil && tc.want == nil) {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// TestValidateMultipleErrors 验证一次调用收集全部错误，
// 路径精确到嵌套字段与数组元素，且顺序稳定（按路径、错误类型排序）。
func TestValidateMultipleErrors(t *testing.T) {
	v := mustValidator(t, complexRuleJSON)
	errs := v.Validate(mustData(t, badDataJSON))
	want := []string{
		"required $.items[1].price",
		"minimum $.items[2].price",
		"additional_property $.unknown",
		"minimum $.user.age",
		"additional_property $.user.extra",
		"required $.user.name",
	}
	if got := summaries(errs); !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestValidateNestedPaths(t *testing.T) {
	v := mustValidator(t, complexRuleJSON)
	errs := v.Validate(mustData(t, `{
	  "user": {"name": "", "age": 30},
	  "items": [{"price": 1}, {"price": 2, "gift": "yes"}]
	}`))
	want := []string{
		"type $.items[1].gift",
		"min_length $.user.name",
	}
	if got := summaries(errs); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestValidateTypeErrorStopsDescent 类型错误后不再深入该节点，
// 避免产生无意义的级联错误。
func TestValidateTypeErrorStopsDescent(t *testing.T) {
	v := mustValidator(t, complexRuleJSON)
	errs := v.Validate(mustData(t, `{"user": "oops", "items": []}`))
	want := []string{
		"min_items $.items",
		"type $.user",
	}
	if got := summaries(errs); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

// TestValidateStableErrorOrder 多次校验同一数据，错误顺序必须完全一致
// （Go map 遍历顺序随机，靠排序保证稳定）。
func TestValidateStableErrorOrder(t *testing.T) {
	v := mustValidator(t, complexRuleJSON)
	data := mustData(t, badDataJSON)
	first := v.Validate(data)
	if len(first) == 0 {
		t.Fatal("expected errors")
	}
	for i := 0; i < 200; i++ {
		if got := v.Validate(data); !reflect.DeepEqual(got, first) {
			t.Fatalf("run %d: unstable error order:\ngot  %v\nwant %v", i, got, first)
		}
	}
}

// TestConcurrentValidate 多 goroutine 并发调用同一个 Validator，
// 配合 go test -race 验证并发安全。
func TestConcurrentValidate(t *testing.T) {
	v := mustValidator(t, complexRuleJSON)
	good := mustData(t, validDataJSON)
	bad := mustData(t, badDataJSON)
	wantGood := v.Validate(good)
	wantBad := v.Validate(bad)
	if len(wantGood) != 0 || len(wantBad) == 0 {
		t.Fatalf("fixture broken: good=%v bad=%v", wantGood, wantBad)
	}

	const goroutines = 16
	const iterations = 250
	var wg sync.WaitGroup
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				data, want := good, wantGood
				if (g+i)%2 == 0 {
					data, want = bad, wantBad
				}
				if got := v.Validate(data); !reflect.DeepEqual(got, want) {
					t.Errorf("goroutine %d iter %d: got %v, want %v", g, i, got, want)
					return
				}
			}
		}(g)
	}
	wg.Wait()
}

func TestValidateJSONHelper(t *testing.T) {
	v := mustValidator(t, `{"type": "string", "minLength": 2}`)
	errs, err := v.ValidateJSON([]byte(`"x"`))
	if err != nil {
		t.Fatalf("ValidateJSON failed: %v", err)
	}
	if len(errs) != 1 || errs[0].Kind != jsonvalidator.ErrMinLength || errs[0].Path != "$" {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if _, err := v.ValidateJSON([]byte(`{oops`)); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestParseHelpers(t *testing.T) {
	rule, err := jsonvalidator.ParseRuleJSON([]byte(`{"type": "boolean"}`))
	if err != nil {
		t.Fatalf("ParseRuleJSON failed: %v", err)
	}
	if rule.Type != jsonvalidator.TypeBoolean {
		t.Fatalf("unexpected rule type: %v", rule.Type)
	}
	if _, err := jsonvalidator.ParseRuleJSON([]byte(`{"type":`)); err == nil {
		t.Fatal("expected error for malformed rule JSON")
	}
	if _, err := jsonvalidator.ParseRuleJSON([]byte(`{"type":"boolean"} {}`)); err == nil {
		t.Fatal("expected error for trailing data in rule JSON")
	}

	data, err := jsonvalidator.ParseDataJSON([]byte(`{"a": [1, "x", true, null]}`))
	if err != nil {
		t.Fatalf("ParseDataJSON failed: %v", err)
	}
	if _, ok := data.(map[string]any); !ok {
		t.Fatalf("unexpected data type: %T", data)
	}
	if _, err := jsonvalidator.ParseDataJSON([]byte(`[1, 2`)); err == nil {
		t.Fatal("expected error for malformed data JSON")
	}
	if _, err := jsonvalidator.ParseDataJSON([]byte(`1 2`)); err == nil {
		t.Fatal("expected error for trailing data in data JSON")
	}
}
