package templateflow

import (
	"fmt"
	htmltmpl "html/template"

	"github.com/otuschhoff/invoice-gen/internal/pdfdom"
	templateload "github.com/otuschhoff/invoice-gen/internal/template"
)

// BuildNamedElements executes a named HTML template, applies CSS styling, and
// converts the resulting document flow into PDFDOM elements.
func BuildNamedElements(templateSource, templateName, cssStyle string, data any) ([]pdfdom.PDFElementNode, error) {
	return BuildNamedElementsWithFuncs(templateSource, templateName, cssStyle, data, nil)
}

// BuildNamedElementsWithFuncs executes a named HTML template with custom
// template functions, applies CSS styling, and converts the resulting document
// flow into PDFDOM elements.
func BuildNamedElementsWithFuncs(templateSource, templateName, cssStyle string, data any, funcs htmltmpl.FuncMap) ([]pdfdom.PDFElementNode, error) {
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
