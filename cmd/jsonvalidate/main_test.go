package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCLI(t *testing.T, args ...string) (int, string, string) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	code := run(args, &stdout, &stderr)
	return code, stdout.String(), stderr.String()
}

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	return path
}

const (
	cliRule      = `{"type":"object","properties":{"name":{"type":"string"}},"required":["name"],"additionalProperties":false}`
	cliDataValid = `{"name":"ok"}`
	cliDataBad   = `{"name":1,"junk":2}`
)

func TestRunDemo(t *testing.T) {
	code, stdout, _ := runCLI(t)
	if code != 1 {
		t.Fatalf("demo exit code = %d, want 1 (demo data is intentionally invalid)", code)
	}
	for _, want := range []string{"invalid: 8 error(s)", "$.user.name", "$.items[2].price", "$.debug"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("demo output missing %q:\n%s", want, stdout)
		}
	}
}

func TestRunWithFilesValid(t *testing.T) {
	rule := writeFile(t, "rule.json", cliRule)
	data := writeFile(t, "data.json", cliDataValid)
	code, stdout, _ := runCLI(t, "-rule", rule, "-data", data)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stdout=%s", code, stdout)
	}
	if !strings.Contains(stdout, "valid") {
		t.Fatalf("unexpected stdout: %s", stdout)
	}
}

func TestRunWithFilesInvalid(t *testing.T) {
	rule := writeFile(t, "rule.json", cliRule)
	data := writeFile(t, "data.json", cliDataBad)
	code, stdout, _ := runCLI(t, "-rule", rule, "-data", data)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%s", code, stdout)
	}
	for _, want := range []string{"$.junk", "$.name"} {
		if !strings.Contains(stdout, want) {
			t.Fatalf("output missing %q:\n%s", want, stdout)
		}
	}
}

func TestRunFlagAndFileErrors(t *testing.T) {
	rule := writeFile(t, "rule.json", cliRule)
	data := writeFile(t, "data.json", cliDataValid)
	badRule := writeFile(t, "bad-rule.json", `{"type":"nope"}`)
	badData := writeFile(t, "bad-data.json", `{oops`)
	missing := filepath.Join(t.TempDir(), "no-such-file.json")

	cases := []struct {
		name string
		args []string
	}{
		{"only rule flag", []string{"-rule", rule}},
		{"only data flag", []string{"-data", data}},
		{"missing rule file", []string{"-rule", missing, "-data", data}},
		{"missing data file", []string{"-rule", rule, "-data", missing}},
		{"invalid rule file", []string{"-rule", badRule, "-data", data}},
		{"invalid data file", []string{"-rule", rule, "-data", badData}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if code, _, _ := runCLI(t, tc.args...); code != 2 {
				t.Fatalf("exit code = %d, want 2", code)
			}
		})
	}
}
