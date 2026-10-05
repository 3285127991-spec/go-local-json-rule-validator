package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFile(t *testing.T, name, content string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestRunBuiltinExample(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run(nil, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1 (built-in data is invalid); stderr: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{"$.age", "$.extra", "$.name", "$.role", "$.tags[1]"} {
		if !strings.Contains(out, want) {
			t.Errorf("output should contain %s:\n%s", want, out)
		}
	}
}

func TestRunWithFilesValid(t *testing.T) {
	rule := writeFile(t, "rule.json", `{
	  "type": "object",
	  "properties": {"name": {"type": "string", "minLength": 1}},
	  "required": ["name"],
	  "additionalProperties": false
	}`)
	data := writeFile(t, "data.json", `{"name": "gopher"}`)

	var stdout, stderr bytes.Buffer
	code := run([]string{"-rule", rule, "-data", data}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr: %s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "valid" {
		t.Fatalf("output = %q, want %q", stdout.String(), "valid")
	}
}

func TestRunWithFilesInvalid(t *testing.T) {
	rule := writeFile(t, "rule.json", `{
	  "type": "object",
	  "properties": {"name": {"type": "string"}},
	  "required": ["name"]
	}`)
	data := writeFile(t, "data.json", `{}`)

	var stdout, stderr bytes.Buffer
	code := run([]string{"-rule", rule, "-data", data}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(stdout.String(), "$.name") {
		t.Fatalf("output should report $.name:\n%s", stdout.String())
	}
}

func TestRunBadRule(t *testing.T) {
	rule := writeFile(t, "rule.json", `{"type": "nope"}`)
	var stdout, stderr bytes.Buffer
	code := run([]string{"-rule", rule}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), `unknown type "nope"`) {
		t.Fatalf("stderr should explain the rule problem:\n%s", stderr.String())
	}
}

func TestRunBadDataJSON(t *testing.T) {
	data := writeFile(t, "data.json", "not json")
	var stdout, stderr bytes.Buffer
	code := run([]string{"-data", data}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "invalid JSON data") {
		t.Fatalf("stderr should mention invalid JSON data:\n%s", stderr.String())
	}
}

func TestRunMissingFile(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := run([]string{"-rule", filepath.Join(t.TempDir(), "missing.json")}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), "error:") {
		t.Fatalf("stderr should report the read error:\n%s", stderr.String())
	}
}
