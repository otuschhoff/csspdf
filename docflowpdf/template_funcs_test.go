package docflowpdf

import (
	"encoding/json"
	htmltmpl "html/template"
	"testing"
	"time"
)

func TestDefaultTemplateFuncMapWithContext_UsesDeterministicNow(t *testing.T) {
	fixed := time.Date(2026, 8, 1, 12, 34, 56, 0, time.UTC)
	funcs := DefaultTemplateFuncMapWithContext(FuncContext{
		DefaultLocale: "en",
		PayloadLocale: "en",
		Now: func() time.Time {
			return fixed
		},
	})

	nowFn, ok := funcs["now"].(func() time.Time)
	if !ok {
		t.Fatalf("expected now helper")
	}
	if got := nowFn(); !got.Equal(fixed) {
		t.Fatalf("unexpected now value: got %v want %v", got, fixed)
	}

	dateOrNowFn, ok := funcs["dateLocalizedOrNow"].(func(any, ...string) string)
	if !ok {
		t.Fatalf("expected dateLocalizedOrNow helper")
	}
	if got := dateOrNowFn(nil); got != "08/01/2026" {
		t.Fatalf("unexpected localized date: got %q", got)
	}
}

func TestDefaultTemplateFuncMap_ContainsGenericHelpers(t *testing.T) {
	funcs := DefaultTemplateFuncMap("en", "en")

	required := []string{"now", "formatDate", "formatDateTime", "dateLocalizedOrNow", "formatLocalizedDate", "formatLocalizedDateOrNow", "firstDate", "workWeek", "sumNumbers", "currency", "currencyCode", "currencySymbol", "currencyName"}
	for _, name := range required {
		if _, ok := funcs[name]; !ok {
			t.Fatalf("expected helper %q to exist", name)
		}
	}

	sumFn, ok := funcs["sumNumbers"].(func(any, string) float64)
	if !ok {
		t.Fatalf("expected sumNumbers helper")
	}
	rows := []any{
		map[string]any{"Hours": 2.5},
		map[string]any{"Hours": 1.5},
	}
	if got := sumFn(rows, "Hours"); got != 4.0 {
		t.Fatalf("unexpected sum: got %v", got)
	}

	_ = htmltmpl.FuncMap(funcs)
}

func TestDefaultTemplateFuncMap_SumNumbersAcceptsJSONAndNativeNumbers(t *testing.T) {
	funcs := DefaultTemplateFuncMap("en", "en")
	sumFn := funcs["sumNumbers"].(func(any, string) float64)
	rows := []any{
		map[string]any{"Amount": json.Number("1.25")},
		map[string]any{"Amount": int64(2)},
		map[string]any{"Amount": uint64(3)},
	}
	if got := sumFn(rows, "Amount"); got != 6.25 {
		t.Fatalf("sumNumbers returned %v, want 6.25", got)
	}
}

func TestDefaultTemplateFuncMapWithContext_CurrencyHelpers(t *testing.T) {
	funcs := DefaultTemplateFuncMapWithContext(FuncContext{DefaultCurrencyCode: "EUR"})

	currencyFn, ok := funcs["currency"].(func(...string) string)
	if !ok {
		t.Fatalf("expected currency helper")
	}
	if got := currencyFn(); got != "EUR" {
		t.Fatalf("unexpected default currency code: %q", got)
	}
	if got := currencyFn("symbol"); got != "\u20ac" {
		t.Fatalf("unexpected currency symbol: %q", got)
	}
	if got := currencyFn("name"); got != "Euro" {
		t.Fatalf("unexpected currency name: %q", got)
	}

	codeFn, ok := funcs["currencyCode"].(func() string)
	if !ok || codeFn() != "EUR" {
		t.Fatalf("expected currencyCode helper to return EUR")
	}
	symbolFn, ok := funcs["currencySymbol"].(func() string)
	if !ok || symbolFn() != "\u20ac" {
		t.Fatalf("expected currencySymbol helper to return euro sign")
	}
	nameFn, ok := funcs["currencyName"].(func() string)
	if !ok || nameFn() != "Euro" {
		t.Fatalf("expected currencyName helper to return Euro")
	}
}

func TestDefaultTemplateFuncMapWithContext_FormatLocalizedDateStyles(t *testing.T) {
	funcs := DefaultTemplateFuncMapWithContext(FuncContext{DefaultLocale: "en", PayloadLocale: "en"})
	formatFn, ok := funcs["formatLocalizedDate"].(func(any, string, ...string) string)
	if !ok {
		t.Fatalf("expected formatLocalizedDate helper")
	}

	date := time.Date(2026, 7, 31, 16, 15, 0, 0, time.UTC)

	if got := formatFn(date, "written-month", "de"); got != "Juli" {
		t.Fatalf("unexpected german month format: %q", got)
	}
	if got := formatFn(date, "local", "en"); got != "July 31 2026" {
		t.Fatalf("unexpected english local format: %q", got)
	}
	if got := formatFn(date, "local", "de"); got != "31. Juli 2026" {
		t.Fatalf("unexpected german local format: %q", got)
	}
	if got := formatFn(date, "layout:2006-01-02", "en"); got != "2026-07-31" {
		t.Fatalf("unexpected custom layout format: %q", got)
	}
	if got := formatFn("2026-07", "written-month", "de"); got != "Juli" {
		t.Fatalf("unexpected YYYY-MM month format: %q", got)
	}
}
