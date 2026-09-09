package csspdf

import (
	"encoding/json"
	"errors"
	"io/fs"
	"path/filepath"
	"reflect"
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

func TestJSONSourceDecodeInto_PreservesLargeIntegerAcrossInputModes(t *testing.T) {
	const largeInteger = "9007199254740993"
	dir := t.TempDir()
	filePath := filepath.Join(dir, "data.json")
	writeFile(t, filePath, `{"id":`+largeInteger+`}`)
	fsys := fstest.MapFS{
		"data.json": &fstest.MapFile{Data: []byte(`{"id":` + largeInteger + `}`)},
	}

	testCases := []struct {
		name   string
		source JSONSource
	}{
		{name: "object", source: JSONSource{Object: map[string]any{"id": int64(9007199254740993)}}},
		{name: "raw", source: JSONSource{Raw: []byte(`{"id":` + largeInteger + `}`)}},
		{name: "text", source: JSONSource{Text: `{"id":` + largeInteger + `}`}},
		{name: "file", source: JSONSource{FilePath: filePath}},
		{name: "fs", source: JSONSource{FS: fsys, FSPath: "data.json"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			var decoded map[string]any
			if err := testCase.source.DecodeInto(&decoded, "source data"); err != nil {
				t.Fatalf("DecodeInto returned error: %v", err)
			}
			number, ok := decoded["id"].(json.Number)
			if !ok {
				t.Fatalf("decoded id type = %T, want json.Number", decoded["id"])
			}
			if number.String() != largeInteger {
				t.Fatalf("decoded id = %q, want %q", number, largeInteger)
			}
		})
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
	if len(assets.Flow.MainFlow) != 1 {
		t.Fatalf("expected 1 inferred mainFlow section, got %d", len(assets.Flow.MainFlow))
	}
	if assets.Flow.MainFlow[0].Template != "doc" {
		t.Fatalf("unexpected inferred mainFlow ordering: %+v", assets.Flow.MainFlow)
	}
	if assets.Flow.PageNumber.Template != "page-number" {
		t.Fatalf("expected inferred pageNumber template page-number, got %q", assets.Flow.PageNumber.Template)
	}
	if assets.Flow.PageNumber.Transformer != "generic" {
		t.Fatalf("expected inferred pageNumber transformer generic, got %q", assets.Flow.PageNumber.Transformer)
	}
}

func TestAssetInputResolveAssets_RejectsUnknownFlowFields(t *testing.T) {
	input := AssetInput{
		HTML: TextSource{Text: `{{define "doc"}}<div>doc</div>{{end}}{{define "page-number"}}<div>1</div>{{end}}`},
		CSS:  TextSource{Text: "@page { size: A4; }"},
		Flow: JSONSource{Text: `{
			"mainFlow":[{"template":"doc","transformer":"generic","unexpected":true}],
			"pageNumber":{"template":"page-number","transformer":"generic"}
		}`},
	}

	_, err := input.ResolveAssets()
	if err == nil || !strings.Contains(err.Error(), "unknown field") {
		t.Fatalf("expected unknown flow field error, got %v", err)
	}
}

func TestAssetInputResolveAssets_HTMLLayersInferSameFlowAsLegacyHTML(t *testing.T) {
	html := `
{{/* {{define "phantom"}} */}}
{{define "doc"}}<div>doc</div>{{end}}
{{define "page-number"}}<div>{{.page.pageNumber}}</div>{{end}}`
	base := AssetInput{
		CSS:  TextSource{Text: "@page { size: A4; }"},
		Flow: JSONSource{Text: `{}`},
	}
	legacy := base
	legacy.HTML = TextSource{Text: html}
	layered := base
	layered.HTMLLayers = []HTMLLayerInput{{Name: "document", Source: TextSource{Text: html}}}

	legacyAssets, err := legacy.ResolveAssets()
	if err != nil {
		t.Fatalf("resolve legacy assets: %v", err)
	}
	layeredAssets, err := layered.ResolveAssets()
	if err != nil {
		t.Fatalf("resolve layered assets: %v", err)
	}
	if len(legacyAssets.Flow.MainFlow) != 1 || legacyAssets.Flow.MainFlow[0].Template != "doc" {
		t.Fatalf("unexpected legacy inferred flow: %+v", legacyAssets.Flow.MainFlow)
	}
	if !reflect.DeepEqual(layeredAssets.Flow, legacyAssets.Flow) {
		t.Fatalf("layered inferred flow differs: got %+v want %+v", layeredAssets.Flow, legacyAssets.Flow)
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
	if len(assets.Flow.MainFlow) != 1 {
		t.Fatalf("expected inferred 1 mainFlow section, got %d", len(assets.Flow.MainFlow))
	}
	if assets.Flow.MainFlow[0].Template != "doc" {
		t.Fatalf("expected inferred doc template, got %+v", assets.Flow.MainFlow)
	}
	if assets.Flow.PageNumber.Template != "page-number" {
		t.Fatalf("expected inferred page-number template, got %q", assets.Flow.PageNumber.Template)
	}
}

func TestAssetInputResolveAssets_FallsBackToLegacyInferenceWithoutDocTemplate(t *testing.T) {
	input := AssetInput{
		HTML: TextSource{Text: `
{{define "summary"}}<div>summary</div>{{end}}
{{define "appendix"}}<div>appendix</div>{{end}}
{{define "page-number"}}<div>{{.page.pageNumber}}</div>{{end}}`},
		CSS:  TextSource{Text: "@page { size: A4; }"},
		Flow: JSONSource{Text: `{}`},
	}

	assets, err := input.ResolveAssets()
	if err != nil {
		t.Fatalf("ResolveAssets returned error: %v", err)
	}
	if len(assets.Flow.MainFlow) != 2 {
		t.Fatalf("expected legacy fallback to infer 2 mainFlow sections, got %d", len(assets.Flow.MainFlow))
	}
	if assets.Flow.MainFlow[0].Template != "summary" || assets.Flow.MainFlow[1].Template != "appendix" {
		t.Fatalf("unexpected legacy fallback ordering: %+v", assets.Flow.MainFlow)
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

func TestAssetInputResolveAssets_CSSLayers_ExplicitMissingLegacyCSSFails(t *testing.T) {
	missingPath := filepath.Join(t.TempDir(), "missing.css")
	input := AssetInput{
		HTML:      TextSource{Text: `{{define "doc"}}<div>ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`},
		CSS:       TextSource{FilePath: missingPath},
		CSSLayers: []CSSLayerInput{{Name: "base", Source: TextSource{Text: "@page { size: A4; }"}}},
		Flow:      JSONSource{Text: `{"mainFlow":[{"template":"doc","transformer":"generic"}],"pageNumber":{"template":"page-number","transformer":"generic"}}`},
	}

	_, err := input.ResolveAssets()
	if err == nil || !strings.Contains(err.Error(), missingPath) {
		t.Fatalf("expected explicit legacy CSS read failure, got %v", err)
	}
}

func TestAssetInputResolveAssets_CSSLayers_UnsetLegacyCSSRemainsOptional(t *testing.T) {
	input := AssetInput{
		HTML:      TextSource{Text: `{{define "doc"}}<div>ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`},
		CSSLayers: []CSSLayerInput{{Name: "base", Source: TextSource{Text: "@page { size: A4; }"}}},
		Flow:      JSONSource{Text: `{"mainFlow":[{"template":"doc","transformer":"generic"}],"pageNumber":{"template":"page-number","transformer":"generic"}}`},
	}

	assets, err := input.ResolveAssets()
	if err != nil {
		t.Fatalf("expected unset legacy CSS to remain optional: %v", err)
	}
	if assets.CSS != "" {
		t.Fatalf("legacy CSS = %q, want empty", assets.CSS)
	}
}

type errorFS struct {
	err error
}

func (f errorFS) Open(string) (fs.File, error) {
	return nil, f.err
}

func TestAssetInputResolveAssets_CSSLayers_ExplicitUnreadableLegacyCSSFails(t *testing.T) {
	input := AssetInput{
		HTML:      TextSource{Text: `{{define "doc"}}<div>ok</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`},
		CSS:       TextSource{FS: errorFS{err: fs.ErrPermission}, FSPath: "doc.css"},
		CSSLayers: []CSSLayerInput{{Name: "base", Source: TextSource{Text: "@page { size: A4; }"}}},
		Flow:      JSONSource{Text: `{"mainFlow":[{"template":"doc","transformer":"generic"}],"pageNumber":{"template":"page-number","transformer":"generic"}}`},
	}

	_, err := input.ResolveAssets()
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("expected explicit legacy CSS permission error, got %v", err)
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
