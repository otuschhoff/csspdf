package docflowpdf

import (
	"errors"
	"fmt"

	"github.com/otuschhoff/csspdf/internal/pdfrender"
)

func configurePageNumberRenderer(layout *pdfrender.LayoutPDF, assets Assets, source map[string]any, input RenderInput, warnf func(string, ...any)) func() error {
	var renderErr error
	layout.SetPageNumRenderer(func(layout *pdfrender.LayoutPDF, page, pageCount int) {
		if page <= 1 {
			return
		}
		elements, err := pageNumberTemplateFlowElements(layout, assets, source, page, pageCount, input)
		if err != nil {
			if !input.AllowPartialRender && renderErr == nil {
				renderErr = fmt.Errorf("page-number template render failed on page %d/%d: %w", page, pageCount, err)
			}
			warnf("failed to render page-number template: %v", err)
			return
		}
		if err := pdfrender.RenderDocTemplateOverlay(layout, elements); err != nil {
			if !input.AllowPartialRender && renderErr == nil {
				renderErr = fmt.Errorf("page-number template flow failed on page %d/%d: %w", page, pageCount, err)
			}
			warnf("failed to render page-number template flow on page %d/%d: %v", page, pageCount, err)
		}
	})
	return func() error { return renderErr }
}

func handleMainFlowError(err error, input RenderInput, warnf func(string, ...any)) error {
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
