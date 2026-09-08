package docflowpdf

import templateload "github.com/otuschhoff/csspdf/internal/templating"

const UnsupportedCSSPropertyCode = templateload.UnsupportedCSSPropertyCode

// CSSSupportDiagnostic describes a syntactically valid CSS declaration that
// the renderer intentionally ignores.
type CSSSupportDiagnostic = templateload.CSSSupportDiagnostic

// SupportedCSSProperties returns the stable mapped CSS property set.
func SupportedCSSProperties() []string {
	return templateload.SupportedCSSProperties()
}

// AnalyzeCSSSupport reports syntactically valid declarations outside the
// renderer's supported CSS subset.
func AnalyzeCSSSupport(cssText string) ([]CSSSupportDiagnostic, error) {
	return templateload.AnalyzeCSSSupport(cssText)
}
