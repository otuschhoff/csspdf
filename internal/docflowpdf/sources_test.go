package docflowpdf

import (
	"testing"
	"testing/fstest"
)

func TestAssetInputResolveAssets_FromInMemorySources(t *testing.T) {
	flow := Flow{
		MainFlow: []Section{{Template: "doc", Transformer: "generic"}},
		PageNumber: Section{Template: "page-number", Transformer: "generic"},
	}
	source := map[string]any{"Company": map[string]any{"Name": "ACME"}}

	input := AssetInput{
		HTML: TextSource{Text: `{{define "doc"}}<div>ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`},
		CSS:  TextSource{Text: "@page { size: A4; }"},
		Flow: JSONSource{Object: flow},
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
