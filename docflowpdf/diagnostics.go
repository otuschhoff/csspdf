package docflowpdf

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// DiagnosticCode is a stable machine-readable render failure category.
type DiagnosticCode string

const (
	DiagnosticInvalidInput DiagnosticCode = "DF001"
	DiagnosticAsset        DiagnosticCode = "DF002"
	DiagnosticTemplate     DiagnosticCode = "DF003"
	DiagnosticLayout       DiagnosticCode = "DF004"
	DiagnosticOutput       DiagnosticCode = "DF005"
	DiagnosticCanceled     DiagnosticCode = "DF006"
	DiagnosticLimit        DiagnosticCode = "DF007"
)

// DiagnosticError adds stable provenance while preserving the original cause.
type DiagnosticError struct {
	Code    DiagnosticCode
	Stage   string
	Section string
	Layer   string
	Page    int
	Err     error
}

func (e *DiagnosticError) Error() string {
	if e == nil {
		return ""
	}
	parts := []string{string(e.Code), e.Stage}
	if e.Section != "" {
		parts = append(parts, "section="+e.Section)
	}
	if e.Layer != "" {
		parts = append(parts, "layer="+e.Layer)
	}
	if e.Page > 0 {
		parts = append(parts, fmt.Sprintf("page=%d", e.Page))
	}
	message := strings.Join(parts, " ")
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	return message
}

func (e *DiagnosticError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func diagnostic(code DiagnosticCode, stage string, err error) error {
	if err == nil {
		return nil
	}
	var existing *DiagnosticError
	if errors.As(err, &existing) {
		return err
	}
	return &DiagnosticError{Code: code, Stage: stage, Err: err}
}

func diagnosticCode(err error, fallback DiagnosticCode) DiagnosticCode {
	switch {
	case errors.Is(err, ErrLimitExceeded):
		return DiagnosticLimit
	case errors.Is(err, context.Canceled), errors.Is(err, context.DeadlineExceeded):
		return DiagnosticCanceled
	default:
		return fallback
	}
}
