package csspdf

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"
)

const (
	DefaultLocale       = "en"
	DefaultCurrencyCode = "EUR"
	DefaultPageFormat   = "A4"

	PageOrientationPortrait  = "portrait"
	PageOrientationLandscape = "landscape"
)

// RenderWithInput renders using a fully specified RenderInput.
func RenderWithInput(input RenderInput) error {
	return RenderWithInputContext(context.Background(), input)
}

// RenderWithInputContext renders using a fully specified input and context.
func RenderWithInputContext(ctx context.Context, input RenderInput) error {
	if strings.TrimSpace(input.OutputPath) == "" {
		return fmt.Errorf("output path is required")
	}
	return RenderToFileContext(ctx, input, input.OutputPath)
}

// RenderToWriter renders and writes a PDF to an io.Writer.
func RenderToWriter(input RenderInput, out io.Writer) error {
	return RenderToWriterContext(context.Background(), input, out)
}

// RenderToWriterContext renders a PDF to out with cancellation and budgets.
func RenderToWriterContext(ctx context.Context, input RenderInput, out io.Writer) error {
	if out == nil {
		return fmt.Errorf("output writer is required")
	}
	if ctx == nil {
		return fmt.Errorf("render context is required")
	}
	limits, err := normalizeRenderLimits(input.Limits)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("render canceled before preparation: %w", err)
	}
	artifact, err := buildArtifactContext(ctx, input, limits)
	if err != nil {
		return diagnostic(diagnosticCode(err, DiagnosticInvalidInput), "preparation", err)
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("render canceled before emission: %w", err)
	}
	if err := emitArtifactContext(ctx, artifact, out, limits.OutputBytes); err != nil {
		return diagnostic(diagnosticCode(err, DiagnosticOutput), "emission", err)
	}
	return nil
}

// RenderToBytes renders and returns a complete PDF byte slice.
func RenderToBytes(input RenderInput) ([]byte, error) {
	return RenderToBytesContext(context.Background(), input)
}

// RenderToBytesContext renders and returns a complete bounded PDF byte slice.
func RenderToBytesContext(ctx context.Context, input RenderInput) ([]byte, error) {
	var buf bytes.Buffer
	if err := RenderToWriterContext(ctx, input, &buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
