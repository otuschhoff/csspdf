package pdfdom

import (
	"testing"

	docformat "github.com/otuschhoff/csspdf/internal/format"
	"github.com/otuschhoff/csspdf/internal/i18n"
)

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
