package main

import (
	"errors"
	"fmt"

	"github.com/otuschhoff/csspdf"
)

func formatCommandError(err error) string {
	var diagnosticErr *csspdf.DiagnosticError
	if !errors.As(err, &diagnosticErr) {
		return err.Error()
	}
	return fmt.Sprintf("%s (%s): %v", diagnosticErr.Code, diagnosticErr.Stage, diagnosticErr.Err)
}
