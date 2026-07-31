// Package invoice provides the application-level use-case orchestration for
// invoice generation. CLI commands depend on this package only; lower-level
// packages are internal implementation details.
package invoice

import (
	"github.com/otuschhoff/invoice-gen/internal/invoice"
	templateload "github.com/otuschhoff/invoice-gen/internal/template"
)

// Page dimension defaults matching the underlying layout engine.
const (
	DocWidth  = templateload.A4Width
	DocHeight = templateload.A4Height
)

// RenderTotalsOptions holds all parameters for the totals PDF rendering use case.
type RenderTotalsOptions struct {
	OutputPath string
	PageWidth  float64
	PageHeight float64
	PageCount  int
}

// RenderTotals generates the totals PDF and writes it to opts.OutputPath.
func RenderTotals(opts RenderTotalsOptions) error {
	return invoice.RenderTotalsPDF(
		opts.OutputPath,
		opts.PageWidth,
		opts.PageHeight,
		opts.PageCount,
	)
}
