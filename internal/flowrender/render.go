package flowrender

import (
	"fmt"
	htmltmpl "html/template"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
	templateload "github.com/otuschhoff/csspdf/internal/templating"
)

// BuildFlowElements executes a named HTML template, applies CSS styling, and
// converts the resulting document flow into PDFDOM elements.
func BuildFlowElements(templateSource, templateName, cssStyle string, data any) ([]pdfdom.PDFElementNode, error) {
	return BuildFlowElementsWithFuncs(templateSource, templateName, cssStyle, data, nil)
}

// BuildFlowElementsWithFuncs executes a named HTML template with custom
// template functions, applies CSS styling, and converts the resulting document
// flow into PDFDOM elements.
func BuildFlowElementsWithFuncs(templateSource, templateName, cssStyle string, data any, funcs htmltmpl.FuncMap) ([]pdfdom.PDFElementNode, error) {
	var (
		htmlStr string
		err     error
	)
	if len(funcs) > 0 {
		htmlStr, err = templateload.ExecuteNamedWithFuncs(templateSource, templateName, data, funcs)
	} else {
		htmlStr, err = templateload.ExecuteNamed(templateSource, templateName, data)
	}
	if err != nil {
		return nil, err
	}

	elements, err := pdfdom.ParseHTMLDocFlow(htmlStr, cssStyle)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s template flow: %w", templateName, err)
	}

	return elements, nil
}

// BuildNamedElements executes a named HTML template, applies CSS styling, and
// converts the resulting document flow into PDFDOM elements.
//
// Deprecated: use BuildFlowElements.
func BuildNamedElements(templateSource, templateName, cssStyle string, data any) ([]pdfdom.PDFElementNode, error) {
	return BuildFlowElements(templateSource, templateName, cssStyle, data)
}

// BuildNamedElementsWithFuncs executes a named HTML template with custom
// template functions, applies CSS styling, and converts the resulting document
// flow into PDFDOM elements.
//
// Deprecated: use BuildFlowElementsWithFuncs.
func BuildNamedElementsWithFuncs(templateSource, templateName, cssStyle string, data any, funcs htmltmpl.FuncMap) ([]pdfdom.PDFElementNode, error) {
	return BuildFlowElementsWithFuncs(templateSource, templateName, cssStyle, data, funcs)
}
