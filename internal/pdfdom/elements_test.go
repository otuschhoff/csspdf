package pdfdom

import (
	"strings"
	"testing"

	docformat "github.com/otuschhoff/csspdf/internal/format"
	"github.com/otuschhoff/csspdf/internal/i18n"
)

func TestElementBuildersReturnValidationErrors(t *testing.T) {
	tests := map[string]func(*ElemImg, PDFNode) error{
		"Add":     func(element *ElemImg, child PDFNode) error { return element.Add(child) },
		"AddLine": func(element *ElemImg, child PDFNode) error { return element.AddLine(child) },
	}
	for name, add := range tests {
		t.Run(name, func(t *testing.T) {
			element := NewElemImg()
			err := add(element, &PDFTextNode{Text: "invalid"})
			if err == nil || !strings.Contains(err.Error(), "img may not have children") {
				t.Fatalf("expected child validation error, got %v", err)
			}
			if len(element.ElementChildren()) != 0 || len(element.ElementChildLineBreaks()) != 0 {
				t.Fatalf("invalid child mutated element: children=%d breaks=%d", len(element.ElementChildren()), len(element.ElementChildLineBreaks()))
			}
		})
	}
}

func TestElemCurrencyValue_FormatEmitsRoundedMonetaryText(t *testing.T) {
	i18nInst, err := i18n.NewFromSource("en", map[string]any{
		"_floatSeparator": map[string]any{"en": "."},
		"_kiloSeparator":  map[string]any{"en": ","},
	})
	if err != nil {
		t.Fatalf("i18n.NewFromSource returned error: %v", err)
	}

	element := NewElemCurrencyValue(999.999)
	if got := element.Format(docformat.New(i18nInst, "CHF")); got != "CHF 1,000.00" {
		t.Fatalf("formatted monetary text = %q, want %q", got, "CHF 1,000.00")
	}
}
