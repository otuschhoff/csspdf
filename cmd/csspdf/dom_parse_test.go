package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunPrintsRenderedDOM(t *testing.T) {
	input := filepath.Join(t.TempDir(), "document.html")
	if err := os.WriteFile(input, []byte(`<!doctype html><p class="message">Hello</p>`), 0o600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if exitCode := runDOMParse("csspdf dom-parse", []string{input}, &stdout, &stderr); exitCode != 0 {
		t.Fatalf("exit code = %d, stderr = %q", exitCode, stderr.String())
	}
	for _, expected := range []string{"DocumentNode", "ElementNode", "message", "Hello"} {
		if !strings.Contains(stdout.String(), expected) {
			t.Fatalf("output does not contain %q: %q", expected, stdout.String())
		}
	}
}

func TestRunRejectsInvalidArguments(t *testing.T) {
	var stderr bytes.Buffer
	if exitCode := runDOMParse("csspdf dom-parse", nil, &bytes.Buffer{}, &stderr); exitCode != 2 {
		t.Fatalf("exit code = %d, want 2", exitCode)
	}
	if !strings.Contains(stderr.String(), "Usage: csspdf dom-parse") {
		t.Fatalf("missing usage message: %q", stderr.String())
	}
}

func TestRunReportsTemplateReadFailure(t *testing.T) {
	var stderr bytes.Buffer
	if exitCode := runDOMParse("csspdf dom-parse", []string{filepath.Join(t.TempDir(), "missing.html")}, &bytes.Buffer{}, &stderr); exitCode != 1 {
		t.Fatalf("exit code = %d, want 1", exitCode)
	}
	if !strings.Contains(stderr.String(), "Error parsing template") {
		t.Fatalf("missing parse error: %q", stderr.String())
	}
}
