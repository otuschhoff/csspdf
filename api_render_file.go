package csspdf

import (
	"context"
	"fmt"
	"strings"

	"github.com/otuschhoff/csspdf/internal/fileout"
)

// RenderToFile renders and atomically writes a PDF to the given file path.
func RenderToFile(input RenderInput, outputPath string) error {
	return RenderToFileContext(context.Background(), input, outputPath)
}

// RenderToFileContext renders and atomically writes a PDF with cancellation.
func RenderToFileContext(ctx context.Context, input RenderInput, outputPath string) error {
	if strings.TrimSpace(outputPath) == "" {
		return fmt.Errorf("output path is required")
	}
	pdf, err := RenderToBytesContext(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to render PDF for %s: %w", outputPath, err)
	}
	return fileout.WriteAtomically(outputPath, pdf)
}
