package templateflow

import (
	"fmt"

	"github.com/otuschhoff/invoice-gen/internal/pdfdom"
	templateload "github.com/otuschhoff/invoice-gen/internal/template"
)

// BuildNamedElements executes a named HTML template, applies CSS styling, and
// converts the resulting document flow into PDFDOM elements.
func BuildNamedElements(templateSource, templateName, cssStyle string, data any) ([]pdfdom.PDFElementNode, error) {
	htmlStr, err := templateload.ExecuteNamed(templateSource, templateName, data)
	if err != nil {
		return nil, err
	}

	elements, err := pdfdom.ParseHTMLDocFlow(htmlStr, cssStyle)
	if err != nil {
		return nil, fmt.Errorf("failed to parse %s template flow: %w", templateName, err)
	}

	return elements, nil
}
