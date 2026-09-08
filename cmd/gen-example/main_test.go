package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRun_LayeredRendersBundledProfile(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "layered.pdf")
	if exitCode := Run("gen-example", []string{"layered", "-o", outputPath}); exitCode != 0 {
		t.Fatalf("Run exit code = %d, want 0", exitCode)
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
	if exitCode := Run("gen-example", []string{"unknown"}); exitCode != 2 {
		t.Fatalf("Run exit code = %d, want 2 for usage error", exitCode)
	}
}
