package pdfdom

import (
	"fmt"

	tmpl "github.com/otuschhoff/csspdf/internal/templating"
	"golang.org/x/net/html"
)

type ParseOptions struct {
	AllowInvalidSpanAttributes bool
	Warnf                      func(string, ...any)
}

func ParseHTMLDocFlowPrepared(htmlString string, stylesheet *tmpl.PreparedStylesheet) ([]PDFElementNode, error) {
	return ParseHTMLDocFlowPreparedWithOptions(htmlString, stylesheet, ParseOptions{})
}

func ParseHTMLDocFlowPreparedWithOptions(htmlString string, stylesheet *tmpl.PreparedStylesheet, options ParseOptions) ([]PDFElementNode, error) {
	document, err := tmpl.ParsePreparedStyledFragment(htmlString, stylesheet)
	if err != nil {
		return nil, err
	}
	return parseHTMLDocFlow(document, options)
}

func parseHTMLDocFlow(document *html.Node, options ParseOptions) ([]PDFElementNode, error) {
	body := tmpl.FindFirst(document, "body")
	if body == nil {
		return nil, fmt.Errorf("no <body> element found in HTML fragment")
	}

	elements := make([]PDFElementNode, 0)
	for _, child := range tmpl.ElemChildren(body) {
		element, supported, err := htmlBuildRootElement(child, options)
		if err != nil {
			return nil, err
		}
		if supported {
			elements = append(elements, element)
		}
	}
	if len(elements) == 0 {
		return nil, fmt.Errorf("no supported top-level elements found in HTML fragment")
	}
	return elements, nil
}
