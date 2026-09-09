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
	if code := runPDFDump("csspdf pdfdump", nil, &bytes.Buffer{}, &stderr); code != 2 {
		t.Fatalf("expected usage exit code 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "Usage: csspdf pdfdump") {
		t.Fatalf("expected usage message, got %q", stderr.String())
	}
}

func TestPDFDumpCLISmoke(t *testing.T) {
	path := filepath.Join(t.TempDir(), "minimal.pdf")
	if err := os.WriteFile(path, []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\n%%EOF"), 0o600); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := runPDFDump("csspdf pdfdump", []string{path}, &bytes.Buffer{}, &stderr); code != 0 {
		t.Fatalf("expected success, got exit code %d: %s", code, stderr.String())
	}
}

func TestPDFDumpCLIMissingFile(t *testing.T) {
	var stderr bytes.Buffer
	path := filepath.Join(t.TempDir(), "missing.pdf")
	if code := runPDFDump("csspdf pdfdump", []string{path}, &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatalf("expected failure, got exit code %d: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "failed to read PDF") {
		t.Fatalf("expected read error, got %q", stderr.String())
	}
}

func TestPDFDumpCLIOverLimit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oversized.pdf")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate((128 << 20) + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	if code := runPDFDump("csspdf pdfdump", []string{path}, &bytes.Buffer{}, &stderr); code != 1 {
		t.Fatalf("expected failure, got exit code %d: %s", code, stderr.String())
	}
	if !strings.Contains(stderr.String(), "exceeds byte limit") {
		t.Fatalf("expected limit error, got %q", stderr.String())
	}
}
