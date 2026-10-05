package jsonvalidator_test

import (
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"jsonvalidator"
)

func mustValidator(t *testing.T, ruleText string) *jsonvalidator.Validator {
	t.Helper()
	rule, err := jsonvalidator.ParseRule(ruleText)
	if err != nil {
		t.Fatalf("ParseRule: %v", err)
	}
	v, err := jsonvalidator.New(rule)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return v
}

func mustData(t *testing.T, text string) any {
	t.Helper()
	data, err := jsonvalidator.ParseData(text)
	if err != nil {
		t.Fatalf("ParseData: %v", err)
	}
	return data
}

// signatures flattens errors to "path|type" strings for easy comparison.
func signatures(errs []jsonvalidator.Error) []string {
	out := make([]string, len(errs))
	for i, e := range errs {
		out[i] = e.Path + "|" + e.Type
	}
	return out
}

func TestValidateTypes(t *testing.T) {
	cases := []struct {
		name    string
		rule    string
		valid   string
		invalid string
		wantGot string // expected type name of the invalid value in the message
	}{
		{"object", `{"type": "object"}`, `{"a": 1}`, `[1, 2]`, "array"},
		{"array", `{"type": "array"}`, `[1, 2]`, `{"a": 1}`, "object"},
		{"string", `{"type": "string"}`, `"hi"`, `42`, "number"},
		{"number", `{"type": "number"}`, `1.5`, `"1"`, "string"},
		{"boolean", `{"type": "boolean"}`, `true`, `0`, "number"},
		{"null vs string", `{"type": "string"}`, `"x"`, `null`, "null"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			v := mustValidator(t, tc.rule)
			if errs := v.Validate(mustData(t, tc.valid)); errs != nil {
				t.Fatalf("valid data rejected: %v", errs)
			}
			errs := v.Validate(mustData(t, tc.invalid))
			if len(errs) != 1 {
				t.Fatalf("want 1 error, got %v", errs)
			}
			if errs[0].Path != "$" || errs[0].Type != jsonvalidator.ErrType {
				t.Fatalf("unexpected error: %+v", errs[0])
			}
			if !strings.Contains(errs[0].Message, "got "+tc.wantGot) {
				t.Fatalf("message %q should mention got %s", errs[0].Message, tc.wantGot)
			}
		})
	}
}

func TestNestedObjectPath(t *testing.T) {
	rule := `{
	  "type": "object",
	  "properties": {
	    "user": {
	      "type": "object",
	      "properties": {"name": {"type": "string", "minLength": 1}},
	      "required": ["name"]
	    }
	  }
	}`
	v := mustValidator(t, rule)

	errs := v.Validate(mustData(t, `{"user": {"name": ""}}`))
	if got, want := signatures(errs), []string{"$.user.name|min_length"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	errs = v.Validate(mustData(t, `{"user": 5}`))
	if got, want := signatures(errs), []string{"$.user|type"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestArrayElementPath(t *testing.T) {
	rule := `{
	  "type": "object",
	  "properties": {
	    "items": {
	      "type": "array",
	      "items": {
	        "type": "object",
	        "properties": {"price": {"type": "number", "minimum": 0}},
	        "required": ["price"]
	      }
	    }
	  }
	}`
	v := mustValidator(t, rule)
	errs := v.Validate(mustData(t, `{"items": [{"price": 1}, {"price": -2}, {}]}`))
	want := []string{"$.items[1].price|minimum", "$.items[2].price|required"}
	if got := signatures(errs); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestRequiredMissing(t *testing.T) {
	v := mustValidator(t, `{
	  "type": "object",
	  "properties": {"user": {"type": "object", "properties": {"name": {"type": "string"}}, "required": ["name"]}},
	  "required": ["user", "id"]
	}`)
	errs := v.Validate(mustData(t, `{"user": {}}`))
	want := []string{"$.id|required", "$.user.name|required"}
	if got := signatures(errs); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if !strings.Contains(errs[0].Message, `"id"`) {
		t.Fatalf("message should name the missing field: %q", errs[0].Message)
	}
}

func TestAdditionalProperties(t *testing.T) {
	strict := mustValidator(t, `{
	  "type": "object",
	  "properties": {"name": {"type": "string"}},
	  "additionalProperties": false
	}`)
	errs := strict.Validate(mustData(t, `{"name": "a", "extra": 1}`))
	if got, want := signatures(errs), []string{"$.extra|unknown_field"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}

	loose := mustValidator(t, `{"type": "object", "properties": {"name": {"type": "string"}}}`)
	if errs := loose.Validate(mustData(t, `{"name": "a", "extra": 1}`)); errs != nil {
		t.Fatalf("extra field should be allowed without additionalProperties=false: %v", errs)
	}

	nested := mustValidator(t, `{
	  "type": "object",
	  "properties": {"user": {"type": "object", "properties": {}, "additionalProperties": false}}
	}`)
	errs = nested.Validate(mustData(t, `{"user": {"ghost": true}}`))
	if got, want := signatures(errs), []string{"$.user.ghost|unknown_field"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}

func TestEnum(t *testing.T) {
	v := mustValidator(t, `{"type": "string", "enum": ["red", "green"]}`)
	if errs := v.Validate(mustData(t, `"red"`)); errs != nil {
		t.Fatalf("enum member rejected: %v", errs)
	}
	errs := v.Validate(mustData(t, `"blue"`))
	if got, want := signatures(errs), []string{"$|enum"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	if !strings.Contains(errs[0].Message, `["red","green"]`) {
		t.Fatalf("message should list allowed values: %q", errs[0].Message)
	}
}

func TestStringLengthBounds(t *testing.T) {
	v := mustValidator(t, `{"type": "string", "minLength": 2, "maxLength": 4}`)
	for _, ok := range []string{`"ab"`, `"abcd"`} {
		if errs := v.Validate(mustData(t, ok)); errs != nil {
			t.Fatalf("%s should be valid: %v", ok, errs)
		}
	}
	if got := signatures(v.Validate(mustData(t, `"a"`))); !reflect.DeepEqual(got, []string{"$|min_length"}) {
		t.Fatalf("got %v", got)
	}
	if got := signatures(v.Validate(mustData(t, `"abcde"`))); !reflect.DeepEqual(got, []string{"$|max_length"}) {
		t.Fatalf("got %v", got)
	}
	// Length is counted in runes, not bytes.
	runeV := mustValidator(t, `{"type": "string", "minLength": 2, "maxLength": 2}`)
	if errs := runeV.Validate(mustData(t, `"你好"`)); errs != nil {
		t.Fatalf("two runes should satisfy minLength=2: %v", errs)
	}
}

func TestNumberBounds(t *testing.T) {
	v := mustValidator(t, `{"type": "number", "minimum": 0, "maximum": 10}`)
	for _, ok := range []string{"0", "10", "5.5"} {
		if errs := v.Validate(mustData(t, ok)); errs != nil {
			t.Fatalf("%s should be valid (bounds are inclusive): %v", ok, errs)
		}
	}
	if got := signatures(v.Validate(mustData(t, "-1"))); !reflect.DeepEqual(got, []string{"$|minimum"}) {
		t.Fatalf("got %v", got)
	}
	if got := signatures(v.Validate(mustData(t, "10.5"))); !reflect.DeepEqual(got, []string{"$|maximum"}) {
		t.Fatalf("got %v", got)
	}
}

func TestArrayBounds(t *testing.T) {
	v := mustValidator(t, `{"type": "array", "items": {"type": "number"}, "minItems": 1, "maxItems": 2}`)
	if errs := v.Validate(mustData(t, "[1]")); errs != nil {
		t.Fatalf("[1] should be valid: %v", errs)
	}
	if got := signatures(v.Validate(mustData(t, "[]"))); !reflect.DeepEqual(got, []string{"$|min_items"}) {
		t.Fatalf("got %v", got)
	}
	if got := signatures(v.Validate(mustData(t, "[1, 2, 3]"))); !reflect.DeepEqual(got, []string{"$|max_items"}) {
		t.Fatalf("got %v", got)
	}
}

func TestMultipleErrorsCollected(t *testing.T) {
	v := mustValidator(t, `{
	  "type": "object",
	  "properties": {
	    "name": {"type": "string", "minLength": 1},
	    "age": {"type": "number", "maximum": 150},
	    "tags": {"type": "array", "items": {"type": "string"}},
	    "active": {"type": "boolean"}
	  },
	  "required": ["name", "age", "email"],
	  "additionalProperties": false
	}`)
	errs := v.Validate(mustData(t, `{
	  "name": "",
	  "age": 200,
	  "tags": ["ok", 7],
	  "active": "yes",
	  "extra": 1
	}`))
	want := []string{
		"$.active|type",
		"$.age|maximum",
		"$.email|required",
		"$.extra|unknown_field",
		"$.name|min_length",
		"$.tags[1]|type",
	}
	if got := signatures(errs); !reflect.DeepEqual(got, want) {
		t.Fatalf("got  %v\nwant %v", got, want)
	}
}

func TestStableErrorOrder(t *testing.T) {
	v := mustValidator(t, `{
	  "type": "object",
	  "properties": {
	    "a": {"type": "string"},
	    "b": {"type": "number"},
	    "c": {"type": "boolean"}
	  },
	  "required": ["a", "b", "c"],
	  "additionalProperties": false
	}`)
	// Many unknown fields: map iteration order is randomized, so an
	// unstable implementation would shuffle the output between calls.
	var sb strings.Builder
	sb.WriteString("{")
	for i := 0; i < 25; i++ {
		if i > 0 {
			sb.WriteString(",")
		}
		fmt.Fprintf(&sb, "\"unknown%02d\": %d", i, i)
	}
	sb.WriteString(",\"a\": 1, \"b\": \"x\"}")
	data := mustData(t, sb.String())

	first := v.Validate(data)
	if len(first) == 0 {
		t.Fatal("expected errors")
	}
	for i, e := range first[1:] {
		prev := first[i]
		if prev.Path > e.Path || (prev.Path == e.Path && prev.Type > e.Type) {
			t.Fatalf("errors not sorted by (path, type): %v then %v", prev, e)
		}
	}
	for run := 0; run < 50; run++ {
		if got := v.Validate(data); !reflect.DeepEqual(got, first) {
			t.Fatalf("run %d differs:\nfirst: %v\ngot:   %v", run, first, got)
		}
	}
}

func TestConcurrentValidate(t *testing.T) {
	v := mustValidator(t, `{
	  "type": "object",
	  "properties": {
	    "user": {
	      "type": "object",
	      "properties": {"name": {"type": "string", "minLength": 1}},
	      "required": ["name"]
	    },
	    "items": {"type": "array", "items": {"type": "number", "minimum": 0}}
	  },
	  "required": ["user"],
	  "additionalProperties": false
	}`)
	good := mustData(t, `{"user": {"name": "gopher"}, "items": [1, 2, 3]}`)
	bad := mustData(t, `{"user": {}, "items": [-1], "extra": true}`)
	wantBad := v.Validate(bad)
	if len(wantBad) == 0 {
		t.Fatal("bad data should produce errors")
	}

	const goroutines = 32
	const iterations = 50
	var wg sync.WaitGroup
	failures := make(chan string, goroutines*iterations)
	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				if errs := v.Validate(good); errs != nil {
					failures <- fmt.Sprintf("goroutine %d: good data rejected: %v", id, errs)
				}
				if errs := v.Validate(bad); !reflect.DeepEqual(errs, wantBad) {
					failures <- fmt.Sprintf("goroutine %d: got %v, want %v", id, errs, wantBad)
				}
			}
		}(g)
	}
	wg.Wait()
	close(failures)
	for f := range failures {
		t.Error(f)
	}
}

func TestParseData(t *testing.T) {
	data, err := jsonvalidator.ParseData(`{"a": [1, "x", true, null]}`)
	if err != nil {
		t.Fatalf("ParseData: %v", err)
	}
	obj, ok := data.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", data)
	}
	arr := obj["a"].([]any)
	if arr[0] != 1.0 || arr[1] != "x" || arr[2] != true || arr[3] != nil {
		t.Fatalf("unexpected decode: %v", arr)
	}
	if _, err := jsonvalidator.ParseData("{not json"); err == nil {
		t.Fatal("expected error for malformed JSON")
	}
	if _, err := jsonvalidator.ParseData("{} {}"); err == nil {
		t.Fatal("expected error for trailing data")
	}
	if _, err := jsonvalidator.ParseData("42"); err != nil {
		t.Fatalf("scalar JSON should parse: %v", err)
	}
}

func TestNewRejectsInvalidRule(t *testing.T) {
	if _, err := jsonvalidator.New(&jsonvalidator.Rule{Type: "wat"}); err == nil {
		t.Fatal("expected error for unknown type")
	}
	if _, err := jsonvalidator.New(nil); err == nil {
		t.Fatal("expected error for nil rule")
	}
}
