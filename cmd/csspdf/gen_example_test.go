package main

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestRun_LayeredRendersBundledProfile(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "layered.pdf")
	if exitCode := runGenExample("csspdf gen-example", []string{"layered", "-o", outputPath}, &bytes.Buffer{}, &bytes.Buffer{}); exitCode != 0 {
		t.Fatalf("runGenExample exit code = %d, want 0", exitCode)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat rendered layered example: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("rendered layered example is empty")
	}
}

func TestRun_UnknownSubcommandReturnsUsageError(t *testing.T) {
	if exitCode := runGenExample("csspdf gen-example", []string{"unknown"}, &bytes.Buffer{}, &bytes.Buffer{}); exitCode != 2 {
		t.Fatalf("runGenExample exit code = %d, want 2 for usage error", exitCode)
	}
}
