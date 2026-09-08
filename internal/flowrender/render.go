package flowrender

import (
	htmltmpl "html/template"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
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
	return BuildFlowElementsFromSourcesWithFuncs([]string{templateSource}, templateName, cssStyle, data, funcs)
}

func BuildFlowElementsFromSourcesWithFuncs(templateSources []string, templateName, cssStyle string, data any, funcs htmltmpl.FuncMap) ([]pdfdom.PDFElementNode, error) {
	return BuildFlowElementsFromSourcesWithOptions(templateSources, templateName, cssStyle, data, funcs, BuildOptions{})
}

func BuildFlowElementsFromSourcesWithOptions(templateSources []string, templateName, cssStyle string, data any, funcs htmltmpl.FuncMap, options BuildOptions) ([]pdfdom.PDFElementNode, error) {
	prepared, err := PrepareFlow(templateSources, cssStyle)
	if err != nil {
		return nil, err
	}
	return prepared.Build(templateName, data, funcs, options)
}
