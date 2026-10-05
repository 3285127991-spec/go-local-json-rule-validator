package jsonvalidator_test

import (
	"os"
	"reflect"
	"testing"
)

// TestExampleFiles keeps examples/ in sync with the library behavior.
func TestExampleFiles(t *testing.T) {
	ruleText, err := os.ReadFile("examples/rule.json")
	if err != nil {
		t.Fatalf("read rule: %v", err)
	}
	dataText, err := os.ReadFile("examples/data.json")
	if err != nil {
		t.Fatalf("read data: %v", err)
	}
	v := mustValidator(t, string(ruleText))
	errs := v.Validate(mustData(t, string(dataText)))
	want := []string{"$.age|maximum", "$.extra|unknown_field"}
	if got := signatures(errs); !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v", got, want)
	}
}
