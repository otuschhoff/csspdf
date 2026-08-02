package docflowpdf

import (
	htmltmpl "html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
)

func TestAssetInputResolveAssets_FromInMemorySources(t *testing.T) {
	flow := Flow{
		MainFlow:   []Section{{Template: "doc", Transformer: "generic"}},
		PageNumber: Section{Template: "page-number", Transformer: "generic"},
	}
	source := map[string]any{"Company": map[string]any{"Name": "ACME"}}

	input := AssetInput{
		HTML:       TextSource{Text: `{{define "doc"}}<div>ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`},
		CSS:        TextSource{Text: "@page { size: A4; }"},
		Flow:       JSONSource{Object: flow},
		SourceData: JSONSource{Object: source},
	}

	assets, err := input.ResolveAssets()
	if err != nil {
		t.Fatalf("ResolveAssets returned error: %v", err)
	}
	if assets.Flow.PageNumber.Template != "page-number" {
		t.Fatalf("unexpected page-number template: %q", assets.Flow.PageNumber.Template)
	}
	if _, ok := assets.SourceData["Company"]; !ok {
		t.Fatalf("expected source data to include Company")
	}
}

func TestAssetInputResolveAssets_FromFS(t *testing.T) {
	fsys := fstest.MapFS{
		"tpl.html": &fstest.MapFile{Data: []byte(`{{define "doc"}}<div>ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`)},
		"tpl.css":  &fstest.MapFile{Data: []byte("@page { size: A4; }")},
		"flow.json": &fstest.MapFile{Data: []byte(`{
			"mainFlow": [{"template": "doc", "transformer": "generic"}],
			"pageNumber": {"template": "page-number", "transformer": "generic"}
		}`)},
		"data.json": &fstest.MapFile{Data: []byte(`{"Invoice": {"ID": "1"}}`)},
	}

	input := AssetInput{
		HTML:       TextSource{FS: fsys, FSPath: "tpl.html"},
		CSS:        TextSource{FS: fsys, FSPath: "tpl.css"},
		Flow:       JSONSource{FS: fsys, FSPath: "flow.json"},
		SourceData: JSONSource{FS: fsys, FSPath: "data.json"},
	}

	assets, err := input.ResolveAssets()
	if err != nil {
		t.Fatalf("ResolveAssets returned error: %v", err)
	}
	if len(assets.Flow.MainFlow) != 1 {
		t.Fatalf("expected one main flow section, got %d", len(assets.Flow.MainFlow))
	}
	if _, ok := assets.SourceData["Invoice"]; !ok {
		t.Fatalf("expected source data to include Invoice")
	}
}

func TestAssetInputResolveAssets_AllowsMissingSourceData(t *testing.T) {
	flow := Flow{
		MainFlow:   []Section{{Template: "doc", Transformer: "generic"}},
		PageNumber: Section{Template: "page-number", Transformer: "generic"},
	}

	input := AssetInput{
		HTML: TextSource{Text: `{{define "doc"}}<div>ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`},
		CSS:  TextSource{Text: "@page { size: A4; }"},
		Flow: JSONSource{Object: flow},
	}

	assets, err := input.ResolveAssets()
	if err != nil {
		t.Fatalf("ResolveAssets returned error: %v", err)
	}
	if assets.SourceData != nil {
		t.Fatalf("expected nil source data when no source data input is provided")
	}
}

func TestAssetInputResolveWithBaseDir_DefaultFiles(t *testing.T) {
	baseDir := t.TempDir()
	html := `{{define "doc"}}<div>ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`
	css := "@page { size: A4; }"
	flow := `{"mainFlow":[{"template":"doc","transformer":"generic"}],"pageNumber":{"template":"page-number","transformer":"generic"}}`
	data := `{"Invoice":{"ID":"42"}}`

	writeFile(t, filepath.Join(baseDir, "doc.html"), html)
	writeFile(t, filepath.Join(baseDir, "doc.css"), css)
	writeFile(t, filepath.Join(baseDir, "flow.json"), flow)
	writeFile(t, filepath.Join(baseDir, "data.json"), data)

	assets, err := (AssetInput{}).ResolveWithBaseDir(baseDir)
	if err != nil {
		t.Fatalf("ResolveWithBaseDir returned error: %v", err)
	}
	if assets.Flow.PageNumber.Template != "page-number" {
		t.Fatalf("unexpected page-number template: %q", assets.Flow.PageNumber.Template)
	}
	if _, ok := assets.SourceData["Invoice"]; !ok {
		t.Fatalf("expected source data to include Invoice")
	}
}

func TestAssetInputResolveWithBaseDir_AllowsPerAssetOverrides(t *testing.T) {
	baseDir := t.TempDir()
	html := `{{define "doc"}}<div>base</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`
	css := "@page { size: A4; }"
	flow := `{"mainFlow":[{"template":"doc","transformer":"generic"}],"pageNumber":{"template":"page-number","transformer":"generic"}}`
	data := `{"Invoice":{"ID":"base"}}`

	writeFile(t, filepath.Join(baseDir, "doc.html"), html)
	writeFile(t, filepath.Join(baseDir, "doc.css"), css)
	writeFile(t, filepath.Join(baseDir, "flow.json"), flow)
	writeFile(t, filepath.Join(baseDir, "data.json"), data)

	overrideData := JSONSource{Object: map[string]any{"Invoice": map[string]any{"ID": "override"}}}
	assets, err := (AssetInput{SourceData: overrideData}).ResolveWithBaseDir(baseDir)
	if err != nil {
		t.Fatalf("ResolveWithBaseDir returned error: %v", err)
	}
	invoice, ok := assets.SourceData["Invoice"].(map[string]any)
	if !ok {
		t.Fatalf("expected invoice map in source data")
	}
	if got, _ := invoice["ID"].(string); got != "override" {
		t.Fatalf("expected overridden ID, got %q", got)
	}
}

func TestAssetInputResolveAssets_AppliesFlowDefaultsWhenEntriesOmitted(t *testing.T) {
	input := AssetInput{
		HTML: TextSource{Text: `
{{define "doc"}}<div>doc</div>{{end}}
{{define "timesheet"}}<div>timesheet</div>{{end}}
{{define "page-number"}}<div>{{.page.pageNumber}}</div>{{end}}`},
		CSS:  TextSource{Text: "@page { size: A4; }"},
		Flow: JSONSource{Text: `{}`},
	}

	assets, err := input.ResolveAssets()
	if err != nil {
		t.Fatalf("ResolveAssets returned error: %v", err)
	}
	if len(assets.Flow.MainFlow) != 2 {
		t.Fatalf("expected 2 inferred mainFlow sections, got %d", len(assets.Flow.MainFlow))
	}
	if assets.Flow.MainFlow[0].Template != "doc" || assets.Flow.MainFlow[1].Template != "timesheet" {
		t.Fatalf("unexpected inferred mainFlow ordering: %+v", assets.Flow.MainFlow)
	}
	if assets.Flow.PageNumber.Template != "page-number" {
		t.Fatalf("expected inferred pageNumber template page-number, got %q", assets.Flow.PageNumber.Template)
	}
	if assets.Flow.PageNumber.Transformer != "generic" {
		t.Fatalf("expected inferred pageNumber transformer generic, got %q", assets.Flow.PageNumber.Transformer)
	}
}

func TestAssetInputResolveWithBaseDir_AllowsMissingFlowFile(t *testing.T) {
	baseDir := t.TempDir()
	html := `{{define "doc"}}<div>doc</div>{{end}}{{define "timesheet"}}<div>ts</div>{{end}}{{define "page-number"}}<div>{{.page.pageNumber}}</div>{{end}}`
	css := "@page { size: A4; }"
	data := `{"Invoice":{"ID":"42"}}`

	writeFile(t, filepath.Join(baseDir, "doc.html"), html)
	writeFile(t, filepath.Join(baseDir, "doc.css"), css)
	writeFile(t, filepath.Join(baseDir, "data.json"), data)

	assets, err := (AssetInput{}).ResolveWithBaseDir(baseDir)
	if err != nil {
		t.Fatalf("ResolveWithBaseDir returned error with missing flow.json: %v", err)
	}
	if len(assets.Flow.MainFlow) != 2 {
		t.Fatalf("expected inferred 2 mainFlow sections, got %d", len(assets.Flow.MainFlow))
	}
	if assets.Flow.PageNumber.Template != "page-number" {
		t.Fatalf("expected inferred page-number template, got %q", assets.Flow.PageNumber.Template)
	}
}

func TestAssetInputResolveAssets_CSSLayers_OrderAndLegacyAppend(t *testing.T) {
	input := AssetInput{
		HTML: TextSource{Text: `{{define "doc"}}<div>ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`},
		CSSLayers: []CSSLayerInput{
			{Name: "base", Source: TextSource{Text: "#a { color: red; }"}},
			{Name: "doc", Source: TextSource{Text: "#a { color: blue; }"}},
		},
		CSS:  TextSource{Text: "#a { color: green; }"},
		Flow: JSONSource{Text: `{"mainFlow":[{"template":"doc","transformer":"generic"}],"pageNumber":{"template":"page-number","transformer":"generic"}}`},
	}

	assets, err := input.ResolveAssets()
	if err != nil {
		t.Fatalf("ResolveAssets returned error: %v", err)
	}
	if len(assets.CSSLayers) != 2 {
		t.Fatalf("expected 2 resolved layers, got %d", len(assets.CSSLayers))
	}
	css, err := effectiveTemplateCSS(assets)
	if err != nil {
		t.Fatalf("effectiveTemplateCSS returned error: %v", err)
	}
	idxBase := strings.Index(css, "color: red")
	idxDoc := strings.Index(css, "color: blue")
	idxLegacy := strings.Index(css, "color: green")
	if idxBase < 0 || idxDoc < 0 || idxLegacy < 0 {
		t.Fatalf("expected composed css to include base/doc/legacy declarations")
	}
	if !(idxBase < idxDoc && idxDoc < idxLegacy) {
		t.Fatalf("expected composed css order base -> doc -> legacy; got %q", css)
	}
}

func TestAssetInputResolveAssets_CSSLayers_OptionalMissingLayerIgnored(t *testing.T) {
	input := AssetInput{
		HTML: TextSource{Text: `{{define "doc"}}<div>ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`},
		CSSLayers: []CSSLayerInput{
			{Name: "optional-missing", Optional: true, Source: TextSource{FilePath: filepath.Join(t.TempDir(), "missing.css")}},
			{Name: "present", Source: TextSource{Text: "#a { color: blue; }"}},
		},
		Flow: JSONSource{Text: `{"mainFlow":[{"template":"doc","transformer":"generic"}],"pageNumber":{"template":"page-number","transformer":"generic"}}`},
	}

	assets, err := input.ResolveAssets()
	if err != nil {
		t.Fatalf("ResolveAssets returned error: %v", err)
	}
	if len(assets.CSSLayers) != 1 {
		t.Fatalf("expected only present layer to resolve, got %d", len(assets.CSSLayers))
	}
	if assets.CSSLayers[0].Name != "present" {
		t.Fatalf("unexpected remaining layer name: %q", assets.CSSLayers[0].Name)
	}
}

func TestAssetInputResolveAssets_CSSLayers_RequiredMissingLayerFails(t *testing.T) {
	input := AssetInput{
		HTML: TextSource{Text: `{{define "doc"}}<div>ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`},
		CSSLayers: []CSSLayerInput{
			{Name: "required-missing", Source: TextSource{FilePath: filepath.Join(t.TempDir(), "missing.css")}},
		},
		Flow: JSONSource{Text: `{"mainFlow":[{"template":"doc","transformer":"generic"}],"pageNumber":{"template":"page-number","transformer":"generic"}}`},
	}

	_, err := input.ResolveAssets()
	if err == nil {
		t.Fatalf("expected ResolveAssets to fail for missing required css layer")
	}
}

func TestAssetInputResolveWithBaseDir_CSSLayersDoNotRequireDocCSS(t *testing.T) {
	baseDir := t.TempDir()
	html := `{{define "doc"}}<div>doc</div>{{end}}{{define "page-number"}}<div>{{.page.pageNumber}}</div>{{end}}`
	data := `{"locale":"en"}`
	layerPath := filepath.Join(baseDir, "corp.css")

	writeFile(t, filepath.Join(baseDir, "doc.html"), html)
	writeFile(t, filepath.Join(baseDir, "data.json"), data)
	writeFile(t, layerPath, "@page { size: A4; } #x { color: red; }")

	assets, err := (AssetInput{
		CSSLayers: []CSSLayerInput{{Name: "corp", Source: TextSource{FilePath: layerPath}}},
	}).ResolveWithBaseDir(baseDir)
	if err != nil {
		t.Fatalf("ResolveWithBaseDir returned error for layer-only css setup: %v", err)
	}
	if len(assets.CSSLayers) != 1 {
		t.Fatalf("expected one resolved css layer, got %d", len(assets.CSSLayers))
	}
}

func TestAssetInputResolveAssets_CSSLayers_MixedSourceTypes(t *testing.T) {
	tempDir := t.TempDir()
	fileLayerPath := filepath.Join(tempDir, "file-layer.css")
	writeFile(t, fileLayerPath, "#x { font-size: 11; }")

	fsys := fstest.MapFS{
		"fs-layer.css": &fstest.MapFile{Data: []byte("#x { font-size: 12; }")},
	}

	input := AssetInput{
		HTML: TextSource{Text: `{{define "doc"}}<div id="x">ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`},
		CSSLayers: []CSSLayerInput{
			{Name: "inline", Source: TextSource{Text: "#x { font-size: 10; }"}},
			{Name: "file", Source: TextSource{FilePath: fileLayerPath}},
			{Name: "fs", Source: TextSource{FS: fsys, FSPath: "fs-layer.css"}},
		},
		Flow: JSONSource{Text: `{"mainFlow":[{"template":"doc","transformer":"generic"}],"pageNumber":{"template":"page-number","transformer":"generic"}}`},
	}

	assets, err := input.ResolveAssets()
	if err != nil {
		t.Fatalf("ResolveAssets returned error: %v", err)
	}
	if len(assets.CSSLayers) != 3 {
		t.Fatalf("expected three resolved layers, got %d", len(assets.CSSLayers))
	}
	css, err := effectiveTemplateCSS(assets)
	if err != nil {
		t.Fatalf("effectiveTemplateCSS returned error: %v", err)
	}
	idx10 := strings.Index(css, "font-size: 10")
	idx11 := strings.Index(css, "font-size: 11")
	idx12 := strings.Index(css, "font-size: 12")
	if idx10 < 0 || idx11 < 0 || idx12 < 0 {
		t.Fatalf("expected composed css to include all mixed-source declarations")
	}
	if !(idx10 < idx11 && idx11 < idx12) {
		t.Fatalf("expected composed css order inline -> file -> fs; got %q", css)
	}
}

func TestAssetInputResolveAssets_HTMLLayers_ComposesSharedWrapperAndContent(t *testing.T) {
	input := AssetInput{
		HTML: TextSource{Text: `
{{define "document-content"}}<div id="doc">Hello {{.Source.Name}}</div>{{end}}`},
		HTMLLayers: []HTMLLayerInput{
			{Name: "wrapper", Source: TextSource{Text: `
{{define "doc"}}<div>{{block "default-letterhead" .}}<div id="letterhead">ACME Corp</div>{{end}}{{block "document-content" .}}<div id="doc">Default Body</div>{{end}}{{block "default-footer" .}}<div id="footer">Default Footer</div>{{end}}</div>{{end}}
{{define "page-number"}}<div id="pn">{{.page.pageNumber}}</div>{{end}}`}},
		},
		CSS: TextSource{Text: "@page { size: A4; }"},
		Flow: JSONSource{Object: Flow{
			MainFlow:   []Section{{Template: "doc", Transformer: "generic", Payload: PayloadConfig{IncludeSource: true}}},
			PageNumber: Section{Template: "page-number", Transformer: "generic"},
		}},
		SourceData: JSONSource{Object: map[string]any{"Name": "Docflow"}},
	}

	assets, err := input.ResolveAssets()
	if err != nil {
		t.Fatalf("ResolveAssets returned error: %v", err)
	}
	if len(assets.HTMLLayers) != 1 {
		t.Fatalf("expected one resolved html layer, got %d", len(assets.HTMLLayers))
	}
	out := executeTemplateSourcesForTest(t, templateSourcesInRenderOrder(assets), "doc", map[string]any{
		"Source": map[string]any{"Name": "Docflow"},
	})
	if !strings.Contains(out, "ACME Corp") {
		t.Fatalf("expected shared letterhead output, got %q", out)
	}
	if !strings.Contains(out, "Hello Docflow") {
		t.Fatalf("expected document-specific content output, got %q", out)
	}
	if !strings.Contains(out, "Default Footer") {
		t.Fatalf("expected shared footer output, got %q", out)
	}
}

func TestAssetInputResolveAssets_HTMLLayers_DocumentOverridesWrapperBlocks(t *testing.T) {
	input := AssetInput{
		HTML: TextSource{Text: `
{{define "document-content"}}<div id="doc">Body</div>{{end}}
{{define "default-footer"}}<div id="footer">Document Footer</div>{{end}}
{{define "page-number"}}<div id="pn">Doc Page {{.page.pageNumber}}</div>{{end}}`},
		HTMLLayers: []HTMLLayerInput{
			{Name: "wrapper", Source: TextSource{Text: `
{{define "doc"}}<div>{{block "default-letterhead" .}}<div id="letterhead">Wrapper Letterhead</div>{{end}}{{block "document-content" .}}<div id="doc">Wrapper Body</div>{{end}}{{block "default-footer" .}}<div id="footer">Wrapper Footer</div>{{end}}</div>{{end}}
{{define "page-number"}}<div id="pn">Wrapper Page</div>{{end}}`}},
		},
		CSS: TextSource{Text: "@page { size: A4; }"},
		Flow: JSONSource{Object: Flow{
			MainFlow:   []Section{{Template: "doc", Transformer: "generic"}},
			PageNumber: Section{Template: "page-number", Transformer: "generic"},
		}},
	}

	assets, err := input.ResolveAssets()
	if err != nil {
		t.Fatalf("ResolveAssets returned error: %v", err)
	}
	docOut := executeTemplateSourcesForTest(t, templateSourcesInRenderOrder(assets), "doc", map[string]any{})
	if !strings.Contains(docOut, "Wrapper Letterhead") {
		t.Fatalf("expected wrapper letterhead in output, got %q", docOut)
	}
	if !strings.Contains(docOut, "Document Footer") {
		t.Fatalf("expected document footer override in output, got %q", docOut)
	}
	if strings.Contains(docOut, "Wrapper Footer") {
		t.Fatalf("expected wrapper footer to be overridden, got %q", docOut)
	}
	pnOut := executeTemplateSourcesForTest(t, templateSourcesInRenderOrder(assets), "page-number", map[string]any{"page": map[string]any{"pageNumber": 2}})
	if !strings.Contains(pnOut, "Doc Page 2") {
		t.Fatalf("expected page-number override output, got %q", pnOut)
	}
}

func TestAssetInputResolveAssets_HTMLLayers_RequiredMissingLayerFails(t *testing.T) {
	input := AssetInput{
		HTML: TextSource{Text: `{{define "doc"}}<div>ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`},
		HTMLLayers: []HTMLLayerInput{
			{Name: "wrapper", Source: TextSource{FilePath: filepath.Join(t.TempDir(), "missing-wrapper.html")}},
		},
		CSS:  TextSource{Text: "@page { size: A4; }"},
		Flow: JSONSource{Text: `{"mainFlow":[{"template":"doc","transformer":"generic"}],"pageNumber":{"template":"page-number","transformer":"generic"}}`},
	}

	_, err := input.ResolveAssets()
	if err == nil {
		t.Fatalf("expected ResolveAssets to fail for missing required html layer")
	}
}

func TestAssetInputResolveAssets_HTMLLayers_OptionalMissingLayerIgnored(t *testing.T) {
	input := AssetInput{
		HTML: TextSource{Text: `{{define "doc"}}<div>ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`},
		HTMLLayers: []HTMLLayerInput{
			{Name: "optional-missing", Optional: true, Source: TextSource{FilePath: filepath.Join(t.TempDir(), "missing-wrapper.html")}},
			{Name: "wrapper", Source: TextSource{Text: `{{define "default-footer"}}<div>Footer</div>{{end}}`}},
		},
		CSS:  TextSource{Text: "@page { size: A4; }"},
		Flow: JSONSource{Text: `{"mainFlow":[{"template":"doc","transformer":"generic"}],"pageNumber":{"template":"page-number","transformer":"generic"}}`},
	}

	assets, err := input.ResolveAssets()
	if err != nil {
		t.Fatalf("ResolveAssets returned error: %v", err)
	}
	if len(assets.HTMLLayers) != 1 {
		t.Fatalf("expected only present html layer to resolve, got %d", len(assets.HTMLLayers))
	}
	if assets.HTMLLayers[0].Name != "wrapper" {
		t.Fatalf("unexpected remaining html layer name: %q", assets.HTMLLayers[0].Name)
	}
}

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
