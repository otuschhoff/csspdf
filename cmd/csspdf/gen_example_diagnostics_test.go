package main

import (
	"errors"
	"strings"
	"testing"

	"github.com/otuschhoff/csspdf"
)

func TestFormatCommandErrorUsesStableDiagnosticHeader(t *testing.T) {
	err := &csspdf.DiagnosticError{Code: csspdf.DiagnosticLayout, Stage: "layout", Err: errors.New("failed")}
	formatted := formatCommandError(err)
	if formatted != "DF004 (layout): failed" || strings.Contains(formatted, "\x1b[") {
		t.Fatalf("unexpected command diagnostic %q", formatted)
	}
}
