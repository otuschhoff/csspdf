package csspdf

import (
	"path/filepath"
	"strings"
	"testing"
)

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
