// Command demo is a small CLI front-end for the jsonvalidator library.
// With no flags it validates a built-in example; use -rule and -data to
// validate JSON files against a rule file.
//
// Exit codes: 0 = valid, 1 = validation errors, 2 = usage/parse error.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"jsonvalidator"
)

const exampleRule = `{
  "type": "object",
  "properties": {
    "name":   {"type": "string", "minLength": 1, "maxLength": 20},
    "age":    {"type": "number", "minimum": 0, "maximum": 150},
    "role":   {"type": "string", "enum": ["admin", "user", "guest"]},
    "tags":   {"type": "array", "items": {"type": "string"}, "minItems": 1, "maxItems": 3},
    "active": {"type": "boolean"}
  },
  "required": ["name", "age"],
  "additionalProperties": false
}
`

const exampleData = `{
  "name": "",
  "age": 200,
  "role": "superuser",
  "tags": ["go", 42],
  "active": true,
  "extra": "not allowed"
}
`

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("demo", flag.ContinueOnError)
	fs.SetOutput(stderr)
	rulePath := fs.String("rule", "", "path to a rule JSON file (built-in example if empty)")
	dataPath := fs.String("data", "", "path to a data JSON file (built-in example if empty)")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	ruleText, err := readInput(*rulePath, exampleRule)
	if err != nil {
		fmt.Fprintln(stderr, "error: "+err.Error())
		return 2
	}
	dataText, err := readInput(*dataPath, exampleData)
	if err != nil {
		fmt.Fprintln(stderr, "error: "+err.Error())
		return 2
	}

	rule, err := jsonvalidator.ParseRule(ruleText)
	if err != nil {
		fmt.Fprintln(stderr, "error: "+err.Error())
		return 2
	}
	data, err := jsonvalidator.ParseData(dataText)
	if err != nil {
		fmt.Fprintln(stderr, "error: "+err.Error())
		return 2
	}

	v, err := jsonvalidator.New(rule)
	if err != nil {
		fmt.Fprintln(stderr, "error: "+err.Error())
		return 2
	}
	errs := v.Validate(data)
	if len(errs) == 0 {
		fmt.Fprintln(stdout, "valid")
		return 0
	}
	fmt.Fprintf(stdout, "invalid: %d error(s)\n", len(errs))
	for _, e := range errs {
		fmt.Fprintf(stdout, "  %s [%s] %s\n", e.Path, e.Type, e.Message)
	}
	return 1
}

func readInput(path, fallback string) (string, error) {
	if path == "" {
		return fallback, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", path, err)
	}
	return string(b), nil
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}
