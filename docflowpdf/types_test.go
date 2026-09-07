package docflowpdf

import (
	"reflect"
	"strings"
	"testing"
)

func TestComposeTemplateCSS_AppendsLegacyAfterLayers(t *testing.T) {
	css, err := composeTemplateCSS(
		[]CSSLayer{
			{Name: "base", CSS: "#x { color: red; }"},
			{Name: "doc", CSS: "#x { color: blue; }"},
		},
		"#x { color: green; }",
	)
	if err != nil {
		t.Fatalf("composeTemplateCSS returned error: %v", err)
	}
	if !(containsInOrder(css, "color: red", "color: blue", "color: green")) {
		t.Fatalf("expected layer and legacy css to be composed in deterministic order, got %q", css)
	}
}

func TestComposeTemplateHTML_AppendsLegacyAfterLayers(t *testing.T) {
	html, err := composeTemplateHTML(
		[]HTMLLayer{
			{Name: "wrapper", HTML: `{{define "doc"}}<div>{{template "document-content" .}}</div>{{end}}`},
			{Name: "defaults", HTML: `{{define "page-number"}}<div>default</div>{{end}}`},
		},
		`{{define "document-content"}}content{{end}}`,
	)
	if err != nil {
		t.Fatalf("composeTemplateHTML returned error: %v", err)
	}
	if !(containsInOrder(html, `define "doc"`, `define "page-number"`, `define "document-content"`)) {
		t.Fatalf("expected layer and legacy html to be composed in deterministic order, got %q", html)
	}
}

func TestEffectiveTemplateHTML_FallsBackToLegacyHTMLWhenNoLayers(t *testing.T) {
	assets := Assets{HTML: `{{define "doc"}}ok{{end}}`}
	html, err := effectiveTemplateHTML(assets)
	if err != nil {
		t.Fatalf("effectiveTemplateHTML returned error: %v", err)
	}
	if !strings.Contains(html, `define "doc"`) {
		t.Fatalf("expected effective html to contain legacy html content")
	}
}

func TestAssetsValidate_AllowsLayeredCSSWithoutLegacy(t *testing.T) {
	assets := Assets{
		HTML:      `{{define "doc"}}<div>ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`,
		CSSLayers: []CSSLayer{{Name: "base", CSS: "@page { size: A4; }"}},
		Flow: Flow{
			MainFlow:   []Section{{Template: "doc", Transformer: "generic"}},
			PageNumber: Section{Template: "page-number", Transformer: "generic"},
		},
	}
	if err := assets.Validate(); err != nil {
		t.Fatalf("expected assets.Validate to accept layered css without legacy css: %v", err)
	}
}

func TestEffectiveTemplateCSS_FallsBackToLegacyCSSWhenNoLayers(t *testing.T) {
	assets := Assets{CSS: "@page { size: A4; margin: 20pt; }"}
	css, err := effectiveTemplateCSS(assets)
	if err != nil {
		t.Fatalf("effectiveTemplateCSS returned error: %v", err)
	}
	if !strings.Contains(css, "@page") {
		t.Fatalf("expected effective css to contain legacy css content")
	}
}

func containsInOrder(haystack string, needles ...string) bool {
	start := 0
	for _, needle := range needles {
		rel := strings.Index(haystack[start:], needle)
		if rel < 0 {
			return false
		}
		idx := start + rel
		start = idx + len(needle)
	}
	return true
}

func TestFlowValidate_RejectsUnsupportedTransformer(t *testing.T) {
	flow := Flow{
		MainFlow:   []Section{{Template: "doc", Transformer: "invoice-doc"}},
		PageNumber: Section{Template: "page-number", Transformer: "generic"},
	}
	if err := flow.Validate(); err == nil {
		t.Fatalf("expected validation error for unsupported transformer")
	}
}

func TestFlowValidate_RejectsInvalidRuntimeExpression(t *testing.T) {
	flow := Flow{
		MainFlow: []Section{{
			Template:    "doc",
			Transformer: "generic",
			Payload: PayloadConfig{
				Runtime: map[string]string{"Layout.Width": "flow.remainingWidth:20,,40"},
			},
		}},
		PageNumber: Section{Template: "page-number", Transformer: "generic"},
	}
	if err := flow.Validate(); err == nil {
		t.Fatalf("expected validation error for invalid runtime expression")
	}
}

func TestFlowValidate_RejectsInvalidPayloadPath(t *testing.T) {
	flow := Flow{
		MainFlow: []Section{{
			Template:    "doc",
			Transformer: "generic",
			Payload: PayloadConfig{
				Runtime: map[string]string{"bad-path": "flow.tableWidth"},
			},
		}},
		PageNumber: Section{Template: "page-number", Transformer: "generic"},
	}
	if err := flow.Validate(); err == nil {
		t.Fatalf("expected validation error for invalid payload path")
	}
}

func TestFlowValidate_RejectsReservedAndConflictingPayloadPaths(t *testing.T) {
	testCases := []struct {
		name    string
		payload PayloadConfig
		want    string
	}{
		{name: "reserved source", payload: PayloadConfig{Static: map[string]any{"Source.customer": "x"}}, want: "reserved root"},
		{name: "reserved page", payload: PayloadConfig{Runtime: map[string]string{"page.width": "flow.tableWidth"}}, want: "reserved root"},
		{name: "same target", payload: PayloadConfig{Runtime: map[string]string{"totals.net": "flow.tableWidth"}, Static: map[string]any{"totals.net": 1}}, want: "defined by both"},
		{name: "path prefix", payload: PayloadConfig{Static: map[string]any{"totals": 1, "totals.net": 2}}, want: "conflict"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			section := Section{Template: "doc", Transformer: "generic", Payload: testCase.payload}
			err := section.Validate()
			if err == nil || !strings.Contains(err.Error(), testCase.want) {
				t.Fatalf("Validate error = %v, want containing %q", err, testCase.want)
			}
		})
	}
}

func TestCloneFlow_DeepCopiesMutablePayloadConfiguration(t *testing.T) {
	original := Flow{MainFlow: []Section{{
		Template:    "doc",
		Transformer: "generic",
		Payload: PayloadConfig{
			Runtime:  map[string]string{"layout.width": "flow.tableWidth"},
			I18nVars: map[string]string{"name": "customer.name"},
			Static: map[string]any{
				"metadata": map[string]any{"labels": []any{"original"}},
			},
		},
	}}}
	cloned := cloneFlow(original)
	cloned.MainFlow[0].Payload.Runtime["layout.width"] = "page.number"
	cloned.MainFlow[0].Payload.I18nVars["name"] = "other.name"
	cloned.MainFlow[0].Payload.Static["metadata"].(map[string]any)["labels"].([]any)[0] = "changed"

	if reflect.DeepEqual(original, cloned) {
		t.Fatalf("expected modified clone to differ")
	}
	if got := original.MainFlow[0].Payload.Runtime["layout.width"]; got != "flow.tableWidth" {
		t.Fatalf("original runtime map changed: %q", got)
	}
	if got := original.MainFlow[0].Payload.Static["metadata"].(map[string]any)["labels"].([]any)[0]; got != "original" {
		t.Fatalf("original nested static value changed: %v", got)
	}
}
