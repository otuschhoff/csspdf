package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPDFDumpCLIUsage(t *testing.T) {
	var stderr bytes.Buffer
	if code := run(nil, &stderr); code != 2 {
		t.Fatalf("expected usage exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage: pdfdump") {
		t.Fatalf("expected usage message, got %q", stderr.String())
	}
}

func TestPDFDumpCLISmoke(t *testing.T) {
	path := filepath.Join(t.TempDir(), "minimal.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n%%EOF"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := run([]string{path}, &stderr); code != 0 {
		t.Fatalf("expected success, got exit code %d: %s", code, stderr.String())
	}
}
