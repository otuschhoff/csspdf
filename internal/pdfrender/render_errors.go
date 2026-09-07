package pdfrender

import "fmt"

func (l *LayoutPDF) recoverableRenderError(message string, err error, args ...any) error {
	args = append(args, err)
	if l.strictRenderErrors {
		return fmt.Errorf(message+": %w", args...)
	}
	l.warnf(message+": %v", args...)
	return nil
}
