package pdfrender

import (
	"strconv"
	"strings"

	"github.com/otuschhoff/csspdf/internal/pdfdom"
)

func floatAttributeOrDefault(elem pdfdom.PDFElementNode, name string, fallback float64, positiveOnly bool) float64 {
	raw, ok := elem.Attribute(name)
	if !ok {
		return fallback
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || (positiveOnly && value <= 0) {
		return fallback
	}
	return value
}
