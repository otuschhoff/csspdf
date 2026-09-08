package docflowpdf

import (
	htmltmpl "html/template"
	"os"
	"strings"
	"testing"
)

func TestAssetInputResolveAssets_HTMLLayers_SharedWrapperSupportsInvoiceAndQuoteContent(t *testing.T) {
	sharedWrapper := `
{{define "doc"}}<div id="page">{{block "default-letterhead" .}}<div id="letterhead">ACME</div>{{end}}{{block "sender-line" .}}<div id="sender">Sender</div>{{end}}{{block "recipient-block" .}}<div id="recipient">Recipient</div>{{end}}{{block "document-content" .}}<div id="content">Default Content</div>{{end}}{{block "default-footer" .}}<div id="footer">Footer</div>{{end}}</div>{{end}}
{{define "page-number"}}<div id="pn">{{.page.pageNumber}}</div>{{end}}`

	invoiceInput := AssetInput{
		HTML: TextSource{Text: `
{{define "document-content"}}<div id="invoice-content">Invoice {{.Source.Invoice.ID}}</div>{{end}}`},
		HTMLLayers: []HTMLLayerInput{{Name: "shared-shell", Source: TextSource{Text: sharedWrapper}}},
		CSS:        TextSource{Text: "@page { size: A4; }"},
		Flow:       JSONSource{Object: Flow{MainFlow: []Section{{Template: "doc", Transformer: "generic", Payload: PayloadConfig{IncludeSource: true}}}, PageNumber: Section{Template: "page-number", Transformer: "generic"}}},
		SourceData: JSONSource{Object: map[string]any{"Invoice": map[string]any{"ID": "INV-42"}}},
	}

	invoiceAssets, err := invoiceInput.ResolveAssets()
	if err != nil {
		t.Fatalf("ResolveAssets returned error for invoice wrapper composition: %v", err)
	}
	invoiceOut := executeTemplateSourcesForTest(t, templateSourcesInRenderOrder(invoiceAssets), "doc", map[string]any{
		"Source": map[string]any{"Invoice": map[string]any{"ID": "INV-42"}},
	})
	if !strings.Contains(invoiceOut, "ACME") || !strings.Contains(invoiceOut, "Invoice INV-42") {
		t.Fatalf("expected invoice output to include shared wrapper + invoice content, got %q", invoiceOut)
	}

	quoteInput := AssetInput{
		HTML: TextSource{Text: `
{{define "document-content"}}<div id="quote-content">Quote {{.Source.Quote.ID}}</div>{{end}}`},
		HTMLLayers: []HTMLLayerInput{{Name: "shared-shell", Source: TextSource{Text: sharedWrapper}}},
		CSS:        TextSource{Text: "@page { size: A4; }"},
		Flow:       JSONSource{Object: Flow{MainFlow: []Section{{Template: "doc", Transformer: "generic", Payload: PayloadConfig{IncludeSource: true}}}, PageNumber: Section{Template: "page-number", Transformer: "generic"}}},
		SourceData: JSONSource{Object: map[string]any{"Quote": map[string]any{"ID": "QT-9"}}},
	}

	quoteAssets, err := quoteInput.ResolveAssets()
	if err != nil {
		t.Fatalf("ResolveAssets returned error for quote wrapper composition: %v", err)
	}
	quoteOut := executeTemplateSourcesForTest(t, templateSourcesInRenderOrder(quoteAssets), "doc", map[string]any{
		"Source": map[string]any{"Quote": map[string]any{"ID": "QT-9"}},
	})
	if !strings.Contains(quoteOut, "ACME") || !strings.Contains(quoteOut, "Quote QT-9") {
		t.Fatalf("expected quote output to include shared wrapper + quote content, got %q", quoteOut)
	}
}

func TestLegacyCSSSourceAsLayer_DefaultName(t *testing.T) {
	layer := LegacyCSSSourceAsLayer(TextSource{FilePath: "/tmp/doc.css"}, "")
	if layer.Name != "legacy-css" {
		t.Fatalf("expected default layer name legacy-css, got %q", layer.Name)
	}
	if layer.Source.FilePath != "/tmp/doc.css" {
		t.Fatalf("expected source filepath to be preserved")
	}
}

func TestMigrateAssetInputLegacyCSSToSingleLayer_ConvertsAndPreservesOrder(t *testing.T) {
	input := AssetInput{
		HTML: TextSource{Text: `{{define "doc"}}<div id="x">ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`},
		CSS:  TextSource{Text: "#x { color: green; }"},
		CSSLayers: []CSSLayerInput{
			{Name: "base", Source: TextSource{Text: "#x { color: red; }"}},
			{Name: "doc", Source: TextSource{Text: "#x { color: blue; }"}},
		},
		Flow: JSONSource{Text: `{"mainFlow":[{"template":"doc","transformer":"generic"}],"pageNumber":{"template":"page-number","transformer":"generic"}}`},
	}

	migrated := MigrateAssetInputLegacyCSSToSingleLayer(input, "legacy")
	if migrated.CSS.IsSet() {
		t.Fatalf("expected legacy CSS field to be cleared after migration")
	}
	if len(migrated.CSSLayers) != 3 {
		t.Fatalf("expected 3 layers after migration, got %d", len(migrated.CSSLayers))
	}
	if migrated.CSSLayers[0].Name != "legacy" {
		t.Fatalf("expected migrated layer to be prepended, got first layer %q", migrated.CSSLayers[0].Name)
	}

	assets, err := migrated.ResolveAssets()
	if err != nil {
		t.Fatalf("ResolveAssets returned error after migration: %v", err)
	}
	css, err := effectiveTemplateCSS(assets)
	if err != nil {
		t.Fatalf("effectiveTemplateCSS returned error after migration: %v", err)
	}
	idxLegacy := strings.Index(css, "color: green")
	idxBase := strings.Index(css, "color: red")
	idxDoc := strings.Index(css, "color: blue")
	if idxLegacy < 0 || idxBase < 0 || idxDoc < 0 {
		t.Fatalf("expected migrated composed css to contain all declarations")
	}
	if !(idxLegacy < idxBase && idxBase < idxDoc) {
		t.Fatalf("expected prepended legacy layer then base then doc; got %q", css)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

func executeTemplateSourcesForTest(t *testing.T, templateSources []string, templateName string, data any) string {
	t.Helper()
	tpl := htmltmpl.New("doc")
	for idx, source := range templateSources {
		var err error
		tpl, err = tpl.Parse(source)
		if err != nil {
			t.Fatalf("failed to parse template source[%d] for test execution: %v", idx, err)
		}
	}
	var b strings.Builder
	if err := tpl.ExecuteTemplate(&b, templateName, data); err != nil {
		t.Fatalf("failed to execute template %q in test: %v", templateName, err)
	}
	return b.String()
}
