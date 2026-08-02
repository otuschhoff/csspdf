package docflowpdf

import (
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
		CSS: TextSource{Text: "#a { color: green; }"},
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}
