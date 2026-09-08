package i18n

import (
	"reflect"
	"strings"
	"testing"
)

func TestNewProvidesLocaleSpecificSeparators(t *testing.T) {
	tests := []struct {
		locale, decimal, thousands string
	}{
		{locale: "en", decimal: ".", thousands: ","},
		{locale: "de", decimal: ",", thousands: "."},
	}
	for _, test := range tests {
		t.Run(test.locale, func(t *testing.T) {
			translations, err := New(test.locale)
			if err != nil {
				t.Fatal(err)
			}
			if translations.Locale() != test.locale || translations.FloatSeparator() != test.decimal || translations.KiloSeparator() != test.thousands {
				t.Fatalf("unexpected locale settings: locale=%q decimal=%q thousands=%q", translations.Locale(), translations.FloatSeparator(), translations.KiloSeparator())
			}
		})
	}
}

func TestNewFromSourceFlattensTranslationsAndAppliesVariables(t *testing.T) {
	translations, err := NewFromSource("en", map[string]any{
		"invoice": map[string]any{
			"title": map[string]any{"en": "Invoice", "de": "Rechnung"},
			"hello": map[string]any{"en": "Hello {{name}}"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := translations.TWithVars("invoice.hello", map[string]string{"name": "Ada"}); got != "Hello Ada" {
		t.Fatalf("translated value = %q", got)
	}
	want := map[string]any{"invoice": map[string]any{"title": "Invoice", "hello": "Hello Ada"}}
	if got := translations.TemplateData(map[string]string{"name": "Ada"}); !reflect.DeepEqual(got, want) {
		t.Fatalf("template data = %#v, want %#v", got, want)
	}
	if got := translations.T("missing.key"); got != "missing.key" {
		t.Fatalf("missing translation = %q", got)
	}
}

func TestNewFromSourceRejectsInvalidLocaleLeaf(t *testing.T) {
	_, err := NewFromSource("en", map[string]any{"title": map[string]any{"en": 42}})
	if err == nil || !strings.Contains(err.Error(), `key "title" locale "en" is not a string`) {
		t.Fatalf("expected contextual locale error, got %v", err)
	}
}
