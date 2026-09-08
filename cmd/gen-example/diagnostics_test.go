package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/otuschhoff/csspdf/docflowpdf"
)

func TestFormatCommandErrorUsesStableDiagnosticHeader(t *testing.T) {
	err := &docflowpdf.DiagnosticError{Code: docflowpdf.DiagnosticLayout, Stage: "layout", Err: errors.New("failed")}
	formatted := formatCommandError(err)
	if formatted != "DF004 (layout): failed" || strings.Contains(formatted, "\x1b[") {
		t.Fatalf("unexpected command diagnostic %q", formatted)
	}
}
