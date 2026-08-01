package docflowpdf

import (
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

	required := []string{"now", "formatDate", "formatDateTime", "dateLocalizedOrNow", "firstDate", "workWeek", "sumNumbers"}
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
