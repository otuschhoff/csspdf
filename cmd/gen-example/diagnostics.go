package main

import (
	"errors"
	"fmt"

	"github.com/otuschhoff/csspdf/docflowpdf"
)

func formatCommandError(err error) string {
	var diagnosticErr *docflowpdf.DiagnosticError
	if !errors.As(err, &diagnosticErr) {
		return err.Error()
	}
	return fmt.Sprintf("%s (%s): %v", diagnosticErr.Code, diagnosticErr.Stage, diagnosticErr.Err)
}
