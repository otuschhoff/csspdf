package csspdf

import (
	"errors"
	"testing"

	"github.com/otuschhoff/csspdf/internal/flowrender"
	"github.com/otuschhoff/csspdf/internal/pdfdump"
	"github.com/otuschhoff/csspdf/internal/pdfrender"
	templateload "github.com/otuschhoff/csspdf/internal/templating"
)

func TestLimitErrorsMatchSharedSentinel(t *testing.T) {
	tests := map[string]error{
		"render budget":   &BudgetError{Stage: "test", Limit: 1},
		"resource bytes":  &LimitError{Resource: "test", Limit: 1},
		"DOM complexity":  &flowrender.ComplexityLimitError{Kind: "nodes", Limit: 1, Actual: 2},
		"pages":           &pdfrender.PageLimitError{Limit: 1, Requested: 2},
		"template output": &templateload.OutputLimitError{Limit: 1},
		"PDF inspection":  &pdfdump.InspectionLimitError{Stage: "input", Limit: 1, Actual: 2},
	}
	for name, err := range tests {
		t.Run(name, func(t *testing.T) {
			wrapped := errors.Join(errors.New("context"), err)
			if !errors.Is(wrapped, ErrLimitExceeded) {
				t.Fatalf("errors.Is(%T, ErrLimitExceeded) = false", err)
			}
		})
	}
}

func TestOperationalBoundaryRecognizesSharedLimitSentinel(t *testing.T) {
	err := &flowrender.ComplexityLimitError{Kind: "nodes", Limit: 1, Actual: 2}
	if !isOperationalBoundaryError(err) {
		t.Fatal("shared limit error was not treated as an operational boundary")
	}
	if code := diagnosticCode(err, DiagnosticLayout); code != DiagnosticLimit {
		t.Fatalf("diagnosticCode() = %s, want %s", code, DiagnosticLimit)
	}
}
