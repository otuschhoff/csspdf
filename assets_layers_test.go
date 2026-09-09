package csspdf

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
		index := start + rel
		start = index + len(needle)
	}
	return true
}
