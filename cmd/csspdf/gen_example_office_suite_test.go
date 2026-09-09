package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadOfficeSuiteCatalogDefinesTwentyUniqueCases(t *testing.T) {
	baseDir, err := resolveOfficeSuiteBaseDir()
	if err != nil {
		t.Fatalf("resolve suite: %v", err)
	}
	catalog, err := loadOfficeSuiteCatalog(baseDir)
	if err != nil {
		t.Fatalf("load suite: %v", err)
	}
	if got := len(sortedOfficeSuiteIDs(catalog.Cases)); got != 20 {
		t.Fatalf("case count = %d, want 20", got)
	}
}

func TestRunOfficeSuiteRendersImageCase(t *testing.T) {
	outputDir := t.TempDir()
	if exitCode := runGenExample("csspdf gen-example", []string{"office-suite", "-case", "04-receipt", "-o", outputDir}, &bytes.Buffer{}, &bytes.Buffer{}); exitCode != 0 {
		t.Fatalf("Run exit code = %d, want 0", exitCode)
	}
	info, err := os.Stat(filepath.Join(outputDir, "04-receipt.pdf"))
	if err != nil {
		t.Fatalf("stat receipt: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("rendered receipt is empty")
	}
}

func TestRunOfficeSuiteVerifiesExpectedFailure(t *testing.T) {
	if exitCode := runGenExample("csspdf gen-example", []string{"office-suite", "-case", "18-invalid-font-size", "-o", t.TempDir()}, &bytes.Buffer{}, &bytes.Buffer{}); exitCode != 0 {
		t.Fatalf("Run exit code = %d, want 0 for caught expected failure", exitCode)
	}
}

func TestListOfficeSuiteCasesLabelsOutcomes(t *testing.T) {
	var output bytes.Buffer
	listOfficeSuiteCases(&output, []officeSuiteCase{
		{ID: "renders", Complexity: "simple", Title: "Success"},
		{ID: "fails", Complexity: "fault", Title: "Failure", ExpectedError: "expected"},
	})
	if got := output.String(); !strings.Contains(got, "renders") || !strings.Contains(got, "expected error") {
		t.Fatalf("list output = %q", got)
	}
}
