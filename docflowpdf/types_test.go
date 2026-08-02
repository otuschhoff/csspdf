package docflowpdf

import (
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

func TestAssetsValidate_AllowsLayeredCSSWithoutLegacy(t *testing.T) {
	assets := Assets{
		HTML: `{{define "doc"}}<div>ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`,
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
