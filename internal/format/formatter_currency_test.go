package format

import (
	"strings"
	"testing"

	"github.com/otuschhoff/csspdf/internal/i18n"
)

func TestFormatCurrency_UsesSymbolForISOCode_DE(t *testing.T) {
	i18nInst, err := i18n.New("de")
	if err != nil {
		t.Fatalf("i18n.New returned error: %v", err)
	}
	f := New(i18nInst, "EUR")
	got := f.FormatCurrency(1234.56)
	if !strings.Contains(got, "\u20ac") {
		t.Fatalf("expected EUR to render with euro symbol, got %q", got)
	}
}

func TestFormatCurrency_UnknownCodeFallsBackToCode(t *testing.T) {
	i18nInst, err := i18n.New("en")
	if err != nil {
		t.Fatalf("i18n.New returned error: %v", err)
	}
	f := New(i18nInst, "XYZ")
	got := f.FormatCurrency(1234.56)
	if !strings.HasPrefix(got, "XYZ ") {
		t.Fatalf("expected unknown code to be preserved, got %q", got)
	}
}
