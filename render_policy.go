package csspdf

import (
	"context"
	"errors"

	"github.com/otuschhoff/csspdf/internal/flowrender"
	"github.com/otuschhoff/csspdf/internal/pdfrender"
)

func configurePageNumberRenderer(ctx context.Context, limits RenderLimits, complexity *flowrender.ComplexityBudget, prepared *flowrender.PreparedFlow, layout *pdfrender.LayoutPDF, assets Assets, source map[string]any, input RenderInput, warnf func(string, ...any)) func() error {
	var renderErr error
	layout.SetPageNumRenderer(func(layout *pdfrender.LayoutPDF, page, pageCount int) {
		if page <= 1 {
			return
		}
		elements, err := pageNumberTemplateFlowElementsPrepared(ctx, limits, complexity, prepared, layout, assets, source, page, pageCount, input)
		if err != nil {
			if (!input.AllowPartialRender || isOperationalBoundaryError(err)) && renderErr == nil {
				renderErr = &DiagnosticError{Code: diagnosticCode(err, DiagnosticTemplate), Stage: "page-number-template", Section: assets.Flow.PageNumber.Template, Page: page, Err: err}
			}
			warnf("failed to render page-number template: %v", err)
			return
		}
		if err := pdfrender.RenderDocTemplateOverlay(layout, elements); err != nil {
			if !input.AllowPartialRender && renderErr == nil {
				renderErr = &DiagnosticError{Code: DiagnosticLayout, Stage: "page-number-layout", Section: assets.Flow.PageNumber.Template, Page: page, Err: err}
			}
			warnf("failed to render page-number template flow on page %d/%d: %v", page, pageCount, err)
		}
	})
	return func() error { return renderErr }
}

func handleMainFlowError(err error, input RenderInput, warnf func(string, ...any)) error {
	if isOperationalBoundaryError(err) {
		return err
	}
	var fatalErr *fatalFlowRenderError
	if errors.As(err, &fatalErr) {
		return fatalErr
	}
	if !input.AllowPartialRender || errors.Is(err, errI18nMacroExpansion) {
		return err
	}
	warnf("%v", err)
	return nil
}

func isOperationalBoundaryError(err error) bool {
	return errors.Is(err, ErrLimitExceeded) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded)
}
