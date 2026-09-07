package main

import (
	"path/filepath"
	"testing"
)

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
