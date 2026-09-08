package errorwrapcheck

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), Analyzer, "a")
}

func TestHasWrappingVerb(t *testing.T) {
	tests := map[string]bool{
		"plain %w":        true,
		"indexed %[1]w":   true,
		"escaped %%w":     false,
		"ordinary %v":     false,
		"percent only %%": false,
	}
	for format, want := range tests {
		if got := hasWrappingVerb(format); got != want {
			t.Fatalf("hasWrappingVerb(%q) = %t, want %t", format, got, want)
		}
	}
}
