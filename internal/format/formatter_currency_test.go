package format

import (
	"math"
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

func TestFormatCurrency_RoundsUsingCurrencyMinorUnits(t *testing.T) {
	testCases := []struct {
		name     string
		locale   string
		currency string
		value    float64
		want     string
	}{
		{name: "positive carry", locale: "en", currency: "CHF", value: 1.999, want: "CHF 2.00"},
		{name: "negative carry", locale: "en", currency: "USD", value: -1.999, want: "$ -2.00"},
		{name: "grouping carry", locale: "en", currency: "EUR", value: 999.999, want: "€ 1,000.00"},
		{name: "zero minor units", locale: "en", currency: "JPY", value: 12.6, want: "¥ 13"},
		{name: "three minor units", locale: "en", currency: "KWD", value: 1.2346, want: "KWD 1.235"},
		{name: "german placement", locale: "de", currency: "EUR", value: 1234.56, want: "1.234,56 €"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			formatter := newTestFormatter(t, testCase.locale, testCase.currency)
			if got := formatter.FormatCurrency(testCase.value); got != testCase.want {
				t.Fatalf("FormatCurrency(%v) = %q, want %q", testCase.value, got, testCase.want)
			}
		})
	}
}

func TestFormatFloat_RoundingAndSpecialValues(t *testing.T) {
	formatter := newTestFormatter(t, "en", "EUR")
	testCases := []struct {
		name     string
		value    float64
		decimals int
		want     string
	}{
		{name: "carry", value: 1.999, decimals: 2, want: "2.00"},
		{name: "negative", value: -1.999, decimals: 2, want: "-2.00"},
		{name: "zero decimals round up", value: 1.5, decimals: 0, want: "2"},
		{name: "zero decimals round down", value: 1.4, decimals: 0, want: "1"},
		{name: "negative zero suppressed", value: -0.004, decimals: 2, want: "0.00"},
		{name: "nan", value: math.NaN(), decimals: 2, want: "NaN"},
		{name: "positive infinity", value: math.Inf(1), decimals: 2, want: "+Inf"},
		{name: "negative infinity", value: math.Inf(-1), decimals: 2, want: "-Inf"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if got := formatter.FormatFloat(testCase.value, testCase.decimals); got != testCase.want {
				t.Fatalf("FormatFloat(%v, %d) = %q, want %q", testCase.value, testCase.decimals, got, testCase.want)
			}
		})
	}
}

func newTestFormatter(t *testing.T, locale, currency string) *Formatter {
	t.Helper()
	i18nInst, err := i18n.NewFromSource(locale, map[string]any{
		"_floatSeparator": map[string]any{"en": ".", "de": ","},
		"_kiloSeparator":  map[string]any{"en": ",", "de": "."},
	})
	if err != nil {
		t.Fatalf("i18n.NewFromSource returned error: %v", err)
	}
	return New(i18nInst, currency)
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
