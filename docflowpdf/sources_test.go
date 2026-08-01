package docflowpdf

import (
	"os"
	"path/filepath"
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

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}
