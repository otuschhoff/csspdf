package pdfdom

import (
	"fmt"
	"strconv"

	tmpl "github.com/otuschhoff/csspdf/internal/templating"
	"golang.org/x/net/html"
)

func htmlBuildCurrencyValue(node *html.Node) (*ElemCurrencyValue, error) {
	raw := htmlAttrValWithFallback(node, "v", "value")
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("<currency-value> invalid v=%q: %w", raw, err)
	}
	element := NewElemCurrencyValue(value)
	copyValueElementAttributes(element, node, "value", "v")
	return element, nil
}

func htmlBuildDateValue(node *html.Node) (*ElemDateValue, error) {
	element := NewElemDateValue(tmpl.AttrVal(node, "value"))
	copyValueElementAttributes(element, node, "value")
	return element, nil
}

func htmlBuildDurationValue(node *html.Node) (*ElemDurationValue, error) {
	raw := htmlAttrValWithFallback(node, "v", "value")
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("<duration-value> invalid v=%q: %w", raw, err)
	}
	element := NewElemDurationValue(value)
	copyValueElementAttributes(element, node, "value", "v")
	return element, nil
}

func htmlBuildManDaysValue(node *html.Node) (*ElemManDaysValue, error) {
	raw := htmlAttrValWithFallback(node, "v", "value")
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return nil, fmt.Errorf("<man-days-value> invalid v=%q: %w", raw, err)
	}
	element := NewElemManDaysValue(value)
	copyValueElementAttributes(element, node, "value", "v")
	return element, nil
}

func copyValueElementAttributes(element PDFElementNode, node *html.Node, excluded ...string) {
	for _, attribute := range node.Attr {
		if !stringInList(attribute.Key, excluded) {
			element.SetAttribute(htmlNormaliseAttrKey(attribute.Key), attribute.Val)
		}
	}
}

func stringInList(value string, candidates []string) bool {
	for _, candidate := range candidates {
		if value == candidate {
			return true
		}
	}
	return false
}

func htmlAttrValWithFallback(node *html.Node, primary, fallback string) string {
	if value := tmpl.AttrVal(node, primary); value != "" {
		return value
	}
	return tmpl.AttrVal(node, fallback)
}
