package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRun_InvoiceRendersBundledProfile(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "invoice.pdf")
	if exitCode := Run("gen-example", []string{"invoice", "-o", outputPath}); exitCode != 0 {
		t.Fatalf("Run exit code = %d, want 0", exitCode)
	}
	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("stat rendered invoice: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("rendered invoice is empty")
	}
}

func TestRun_InvoiceRenderFailureReturnsNonzero(t *testing.T) {
	outputPath := filepath.Join(t.TempDir(), "invoice.pdf")
	exitCode := Run("gen-example", []string{
		"invoice",
		"-o", outputPath,
		"-page-format", "not-a-page-format",
	})
	if exitCode != 1 {
		t.Fatalf("Run exit code = %d, want 1 for render failure", exitCode)
	}
}
