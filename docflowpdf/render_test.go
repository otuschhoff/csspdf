package docflowpdf

import (
	"bytes"
	htmltmpl "html/template"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/otuschhoff/csspdf/internal/format"
	"github.com/otuschhoff/csspdf/internal/i18n"
	"github.com/otuschhoff/csspdf/internal/pdfdom"
	"github.com/otuschhoff/csspdf/internal/pdfrender"
	templateload "github.com/otuschhoff/csspdf/internal/templating"
)

func minimalAssets() Assets {
	return Assets{
		HTML: `
{{define "doc"}}<div>Hello {{.Source.Name}}</div>{{end}}
{{define "page-number"}}<div>{{.Page}}/{{.Total}}</div>{{end}}`,
		CSS: "@page { size: A4; margin: 20pt; }",
		Flow: Flow{
			MainFlow: []Section{{
				Template:    "doc",
				Transformer: "generic",
				Payload: PayloadConfig{
					IncludeSource: true,
				},
			}},
			PageNumber: Section{
				Template:    "page-number",
				Transformer: "generic",
				Payload:     PayloadConfig{Runtime: map[string]string{"Page": "page.number", "Total": "page.total"}},
			},
		},
		SourceData: map[string]any{"Name": "Docflow", "locale": "en"},
	}
}

func TestRenderToBytes_ReturnsPDFBytes(t *testing.T) {
	b, err := RenderToBytes(RenderInput{
		Assets:              minimalAssets(),
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
	})
	if err != nil {
		t.Fatalf("RenderToBytes returned error: %v", err)
	}
	if len(b) == 0 {
		t.Fatalf("expected non-empty PDF bytes")
	}
	if !bytes.HasPrefix(b, []byte("%PDF")) {
		t.Fatalf("expected PDF header, got %q", string(b[:4]))
	}
}

func TestRender_OptionsAPI_WritesPDF(t *testing.T) {
	assets := minimalAssets()
	outPath := filepath.Join(t.TempDir(), "out.pdf")
	err := Render(
		outPath,
		WithAssets(assets),
		WithPageSize(595.28, 841.89),
		WithDefaultLocale("en"),
		WithDefaultCurrencyCode("EUR"),
	)
	if err != nil {
		t.Fatalf("Render returned error: %v", err)
	}
	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output pdf: %v", err)
	}
	if len(b) == 0 || !bytes.HasPrefix(b, []byte("%PDF")) {
		t.Fatalf("expected PDF output file")
	}
}

func TestResolvePageDimensions_DefaultsToA4Portrait(t *testing.T) {
	width, height, err := resolvePageDimensions(RenderInput{})
	if err != nil {
		t.Fatalf("resolvePageDimensions returned error: %v", err)
	}
	if width != 595.28 || height != 841.89 {
		t.Fatalf("unexpected A4 portrait defaults: got %fx%f", width, height)
	}
}

func TestResolvePageDimensions_NamedFormatLandscape(t *testing.T) {
	width, height, err := resolvePageDimensions(RenderInput{
		PageFormat:      "A5",
		PageOrientation: "landscape",
	})
	if err != nil {
		t.Fatalf("resolvePageDimensions returned error: %v", err)
	}
	if width != 595.28 || height != 419.53 {
		t.Fatalf("unexpected A5 landscape dimensions: got %fx%f", width, height)
	}
}

func TestResolvePageDimensions_InvalidFormatOrOrientation(t *testing.T) {
	if _, _, err := resolvePageDimensions(RenderInput{PageFormat: "bogus"}); err == nil {
		t.Fatalf("expected error for unsupported format")
	}
	if _, _, err := resolvePageDimensions(RenderInput{PageOrientation: "sideways"}); err == nil {
		t.Fatalf("expected error for unsupported orientation")
	}
}

func TestRender_OptionsAPI_RequiresOutputPath(t *testing.T) {
	err := Render("", WithAssets(minimalAssets()))
	if err == nil || !strings.Contains(err.Error(), "output path is required") {
		t.Fatalf("expected output path validation error, got %v", err)
	}
}

func TestRenderToWriter_WritesPDF(t *testing.T) {
	var out bytes.Buffer
	err := RenderToWriter(RenderInput{
		Assets:              minimalAssets(),
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
	}, &out)
	if err != nil {
		t.Fatalf("RenderToWriter returned error: %v", err)
	}
	if out.Len() == 0 {
		t.Fatalf("expected rendered PDF output")
	}
}

type testLogger struct {
	warnings []string
}

func (l *testLogger) Warnf(format string, args ...any) {
	l.warnings = append(l.warnings, format)
}

func TestRender_UsesLoggerForNonFatalWarnings(t *testing.T) {
	assets := minimalAssets()
	assets.Flow.MainFlow[0].Template = "missing-doc-template"

	logger := &testLogger{}
	var out bytes.Buffer
	err := RenderToWriter(RenderInput{
		Assets:              assets,
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
		Logger:              logger,
	}, &out)
	if err != nil {
		t.Fatalf("RenderToWriter returned error: %v", err)
	}
	if len(logger.warnings) == 0 {
		t.Fatalf("expected warning log for main-flow render issue")
	}
}

func TestRenderToBytes_SupportsLayeredCSSWithoutLegacyCSS(t *testing.T) {
	assets := minimalAssets()
	assets.CSS = ""
	assets.CSSLayers = []CSSLayer{
		{Name: "base", CSS: "@page { size: A4; margin: 24pt; }"},
		{Name: "doc", CSS: "#body { color: #333; }"},
	}

	b, err := RenderToBytes(RenderInput{
		Assets:              assets,
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
	})
	if err != nil {
		t.Fatalf("RenderToBytes returned error: %v", err)
	}
	if len(b) == 0 || !bytes.HasPrefix(b, []byte("%PDF")) {
		t.Fatalf("expected PDF bytes from layered CSS input")
	}
}

func TestRenderToBytes_WarnsWhenUsingCSSLayersAndLegacyCSS(t *testing.T) {
	assets := minimalAssets()
	assets.CSSLayers = []CSSLayer{{Name: "base", CSS: "@page { size: A4; margin: 25pt; }"}}
	assets.CSS = "@page { margin: 20pt; }"

	logger := &testLogger{}
	_, err := RenderToBytes(RenderInput{
		Assets:              assets,
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
		Logger:              logger,
	})
	if err != nil {
		t.Fatalf("RenderToBytes returned error: %v", err)
	}

	found := false
	for _, warning := range logger.warnings {
		if strings.Contains(warning, "both CSSLayers and legacy CSS are set") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected mixed-mode CSS warning to be emitted")
	}
}

func TestRenderToBytes_LogsResolvedCSSLayerOrderWhenLayersPresent(t *testing.T) {
	assets := minimalAssets()
	assets.CSS = ""
	assets.CSSLayers = []CSSLayer{
		{Name: "corporate-base", CSS: "@page { size: A4; margin: 21pt; }"},
		{Name: "invoice-doc", CSS: "#body { color: #333; }"},
	}

	logger := &testLogger{}
	_, err := RenderToBytes(RenderInput{
		Assets:              assets,
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
		Logger:              logger,
	})
	if err != nil {
		t.Fatalf("RenderToBytes returned error: %v", err)
	}

	found := false
	for _, warning := range logger.warnings {
		if strings.Contains(warning, "resolved CSS layer order") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected resolved CSS layer order diagnostic warning")
	}
}

func TestFormatResolvedCSSLayers_IncludesLegacyWhenPresent(t *testing.T) {
	got := formatResolvedCSSLayers([]CSSLayer{{Name: "base"}, {Name: "doc"}}, true)
	if got != "base -> doc -> legacy-css" {
		t.Fatalf("unexpected layer order format: %q", got)
	}
}

func TestLayeredCSS_ParserMappedPropertyOverrideOrder(t *testing.T) {
	assets := Assets{
		HTML: `{{define "doc"}}<div id="x">Hello</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`,
		CSSLayers: []CSSLayer{
			{Name: "base", CSS: "#x { font-size: 10; color: #111111; }"},
			{Name: "doc", CSS: "#x { font-size: 12; color: #222222; }"},
		},
		CSS: "#x { font-size: 14; color: #333333; }",
		Flow: Flow{
			MainFlow:   []Section{{Template: "doc", Transformer: "generic"}},
			PageNumber: Section{Template: "page-number", Transformer: "generic"},
		},
	}

	effectiveCSS, err := effectiveTemplateCSS(assets)
	if err != nil {
		t.Fatalf("effectiveTemplateCSS returned error: %v", err)
	}

	elements, err := pdfdom.ParseHTMLDocFlow(`<div id="x">Hello</div>`, effectiveCSS)
	if err != nil {
		t.Fatalf("ParseHTMLDocFlow returned error: %v", err)
	}
	if len(elements) != 1 {
		t.Fatalf("expected one root element, got %d", len(elements))
	}
	fontSize, ok := elements[0].Attribute("font-size")
	if !ok {
		t.Fatalf("expected font-size attribute to be mapped")
	}
	fontColor, ok := elements[0].Attribute("font-color")
	if !ok {
		t.Fatalf("expected font-color attribute to be mapped")
	}
	if fontSize != "14" {
		t.Fatalf("expected final font-size override to be 14, got %q", fontSize)
	}
	if fontColor != "#333333" {
		t.Fatalf("expected final font-color override to be #333333, got %q", fontColor)
	}
}

func TestRender_UsesContextFuncFactoryNow(t *testing.T) {
	assets := minimalAssets()
	assets.HTML = `{{define "doc"}}<div>{{fmtNow}}</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`
	fixed := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	called := false
	_, err := RenderToBytes(RenderInput{
		Assets:              assets,
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
		Now: func() time.Time {
			return fixed
		},
		FuncMapFactoryEx: func(ctx FuncContext) htmltmpl.FuncMap {
			called = true
			return htmltmpl.FuncMap{
				"fmtNow": func() string { return ctx.Now().Format(time.RFC3339) },
			}
		},
	})
	if err != nil {
		t.Fatalf("RenderToBytes returned error: %v", err)
	}
	if !called {
		t.Fatalf("expected context function factory to be called")
	}
}

func TestRenderToFile_RequiresPath(t *testing.T) {
	err := RenderToFile(RenderInput{Assets: minimalAssets()}, "")
	if err == nil || !strings.Contains(err.Error(), "output path is required") {
		t.Fatalf("expected output path validation error, got %v", err)
	}
}

func minimalI18nSourceForLocale(locale string) map[string]any {
	return map[string]any{
		"_floatSeparator": map[string]any{locale: "."},
		"_kiloSeparator":  map[string]any{locale: ","},
		"invoice": map[string]any{
			"title": map[string]any{locale: "Invoice"},
		},
	}
}

func TestRender_I18nSource_Object(t *testing.T) {
	_, err := RenderToBytes(RenderInput{
		Assets:              minimalAssets(),
		DefaultLocale:       "zz",
		DefaultCurrencyCode: "EUR",
		I18nSource:          JSONSource{Object: minimalI18nSourceForLocale("zz")},
	})
	if err != nil {
		t.Fatalf("expected render to succeed with object i18n source: %v", err)
	}
}

func TestRender_I18nSource_Text(t *testing.T) {
	_, err := RenderToBytes(RenderInput{
		Assets:              minimalAssets(),
		DefaultLocale:       "yy",
		DefaultCurrencyCode: "EUR",
		I18nSource:          JSONSource{Text: `{"_floatSeparator":{"yy":"."},"_kiloSeparator":{"yy":","},"invoice":{"title":{"yy":"Invoice"}}}`},
	})
	if err != nil {
		t.Fatalf("expected render to succeed with text i18n source: %v", err)
	}
}

func TestRender_I18nSource_FilePath(t *testing.T) {
	tempDir := t.TempDir()
	filePath := tempDir + "/i18n.json"
	content := `{"_floatSeparator":{"xx":"."},"_kiloSeparator":{"xx":","},"invoice":{"title":{"xx":"Invoice"}}}`
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write temp i18n file: %v", err)
	}

	_, err := RenderToBytes(RenderInput{
		Assets:              minimalAssets(),
		DefaultLocale:       "xx",
		DefaultCurrencyCode: "EUR",
		I18nSource:          JSONSource{FilePath: filePath},
	})
	if err != nil {
		t.Fatalf("expected render to succeed with file i18n source: %v", err)
	}
}

func TestRender_I18nSource_FSPath(t *testing.T) {
	fs := fstest.MapFS{
		"cfg/i18n.json": &fstest.MapFile{Data: []byte(`{"_floatSeparator":{"xy":"."},"_kiloSeparator":{"xy":","},"invoice":{"title":{"xy":"Invoice"}}}`)},
	}

	_, err := RenderToBytes(RenderInput{
		Assets:              minimalAssets(),
		DefaultLocale:       "xy",
		DefaultCurrencyCode: "EUR",
		I18nSource:          JSONSource{FS: fs, FSPath: "cfg/i18n.json"},
	})
	if err != nil {
		t.Fatalf("expected render to succeed with fs i18n source: %v", err)
	}
}

func TestTransformGenericSection_InjectsImplicitPageObject(t *testing.T) {
	i18nInst, err := i18n.New("en")
	if err != nil {
		t.Fatalf("failed to create i18n instance: %v", err)
	}
	formatter := format.New(i18nInst, "EUR")
	pageSettings := templateload.PageSettings{
		Width:  595.28,
		Height: 841.89,
		Margins: templateload.PageMargins{
			Top:    90,
			Right:  55,
			Bottom: 20,
			Left:   55,
		},
	}
	layout, err := pdfrender.NewLayoutPDF(pageSettings, pageSettings, i18nInst, formatter)
	if err != nil {
		t.Fatalf("failed to create layout: %v", err)
	}
	layout.StartFlow()
	layout.BeginPage(1)
	layout.EnsureTotalPagesAtLeast(3)

	section := Section{
		Template:    "doc",
		Transformer: "generic",
		Payload: PayloadConfig{
			IncludeSource: true,
		},
	}
	payload, err := transformGenericSection(section, transformContext{
		Layout: layout,
		Source: map[string]any{"locale": "en"},
		Page:   2,
		Total:  3,
		Input:  RenderInput{PageOrientation: "portrait"},
	})
	if err != nil {
		t.Fatalf("transformGenericSection returned error: %v", err)
	}

	page, ok := payload["page"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload.page map, got %T", payload["page"])
	}
	if got := page["pageNumber"]; got != 2 {
		t.Fatalf("expected page.pageNumber=2, got %v", got)
	}
	if got := page["pageNumberTotal"]; got != 3 {
		t.Fatalf("expected page.pageNumberTotal=3, got %v", got)
	}
	if got := page["orientation"]; got != "portrait" {
		t.Fatalf("expected page.orientation=portrait, got %v", got)
	}
	if got := page["marginLeft"]; got != 55.0 {
		t.Fatalf("expected page.marginLeft=55, got %v", got)
	}
	if got := page["marginRight"]; got != 55.0 {
		t.Fatalf("expected page.marginRight=55, got %v", got)
	}
	if got := page["width"]; got != 595.28 {
		t.Fatalf("expected page.width=595.28, got %v", got)
	}
}

func TestRender_PageTemplateCanUseImplicitPageObjectWithoutRuntimeConfig(t *testing.T) {
	assets := minimalAssets()
	assets.HTML = `
{{define "doc"}}<div>{{.page.orientation}} {{.page.width}} {{.page.marginLeft}}</div>{{end}}
{{define "page-number"}}<div>{{.page.pageNumber}} / {{.page.pageNumberTotal}}</div>{{end}}`
	assets.Flow.PageNumber.Payload.Runtime = map[string]string{}

	_, err := RenderToBytes(RenderInput{
		Assets:              assets,
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
	})
	if err != nil {
		t.Fatalf("expected render to succeed without pageNumber runtime wiring: %v", err)
	}
}

func TestRenderToBytes_AppliesFlowDefaultsWhenFlowEntriesMissing(t *testing.T) {
	assets := Assets{
		HTML: `
{{define "doc"}}<div>Hello {{.Source.Name}}</div>{{end}}
{{define "timesheet"}}<div>{{.Source.Name}}</div>{{end}}
{{define "page-number"}}<div>{{.page.pageNumber}}/{{.page.pageNumberTotal}}</div>{{end}}`,
		CSS:  "@page { size: A4; margin: 20pt; }",
		Flow: Flow{},
		SourceData: map[string]any{
			"Name":   "Docflow",
			"locale": "en",
		},
	}

	b, err := RenderToBytes(RenderInput{
		Assets:              assets,
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
	})
	if err != nil {
		t.Fatalf("expected render to succeed with inferred flow defaults: %v", err)
	}
	if len(b) == 0 || !bytes.HasPrefix(b, []byte("%PDF")) {
		t.Fatalf("expected rendered PDF output")
	}
}

func TestResolveFontRegistrations_UsesBaseDirFontsWhenNoOverride(t *testing.T) {
	baseDir := t.TempDir()
	fontsDir := filepath.Join(baseDir, "fonts")
	if err := os.MkdirAll(fontsDir, 0755); err != nil {
		t.Fatalf("failed to create fonts dir: %v", err)
	}
	fontPath := filepath.Join(fontsDir, "MyFont.ttf")
	if err := os.WriteFile(fontPath, []byte("dummy"), 0644); err != nil {
		t.Fatalf("failed to write font file: %v", err)
	}

	regs, err := resolveFontRegistrations(RenderInput{AssetBaseDir: baseDir})
	if err != nil {
		t.Fatalf("resolveFontRegistrations returned error: %v", err)
	}
	if len(regs) != 1 {
		t.Fatalf("expected one discovered font registration, got %d", len(regs))
	}
	if regs[0].Family != "MyFont" {
		t.Fatalf("unexpected discovered family: %q", regs[0].Family)
	}
}

func TestResolveFontRegistrations_PrefersExplicitOverrides(t *testing.T) {
	overrides := []FontRegistration{{Family: "Override", Sources: []string{"/tmp/override.ttf"}}}
	regs, err := resolveFontRegistrations(RenderInput{AssetBaseDir: t.TempDir(), FontRegistrations: overrides})
	if err != nil {
		t.Fatalf("resolveFontRegistrations returned error: %v", err)
	}
	if len(regs) != 1 || regs[0].Family != "Override" {
		t.Fatalf("expected explicit override registration to be used")
	}
}

func TestResolveFontRegistrations_DerivesStyleFromFilenameSuffix(t *testing.T) {
	baseDir := t.TempDir()
	fontsDir := filepath.Join(baseDir, "fonts")
	if err := os.MkdirAll(fontsDir, 0755); err != nil {
		t.Fatalf("failed to create fonts dir: %v", err)
	}
	fixtures := []string{"Helvetica-Bold.ttf", "Helvetica-Italic.ttf", "Helvetica-BoldItalic.ttf", "Futura-Medium.ttf"}
	for _, fixture := range fixtures {
		if err := os.WriteFile(filepath.Join(fontsDir, fixture), []byte("dummy"), 0644); err != nil {
			t.Fatalf("failed to write fixture %q: %v", fixture, err)
		}
	}

	regs, err := resolveFontRegistrations(RenderInput{AssetBaseDir: baseDir})
	if err != nil {
		t.Fatalf("resolveFontRegistrations returned error: %v", err)
	}

	got := map[string]bool{}
	for _, reg := range regs {
		got[reg.Family+"|"+reg.Style] = true
	}

	if !got["Helvetica|B"] {
		t.Fatalf("expected Helvetica-Bold.ttf to normalize to Helvetica/B")
	}
	if !got["Helvetica|I"] {
		t.Fatalf("expected Helvetica-Italic.ttf to normalize to Helvetica/I")
	}
	if !got["Helvetica|BI"] {
		t.Fatalf("expected Helvetica-BoldItalic.ttf to normalize to Helvetica/BI")
	}
	if !got["Futura-Medium|"] {
		t.Fatalf("expected Futura-Medium.ttf to remain regular style")
	}
}

func TestNormalizeFontRegistration_NormalizesTextualStyleCodes(t *testing.T) {
	reg := normalizeFontRegistration(FontRegistration{Family: "Body", Style: "bold italic", Sources: []string{"/tmp/body.ttf"}})
	if reg.Family != "Body" {
		t.Fatalf("unexpected family normalization: %q", reg.Family)
	}
	if reg.Style != "BI" {
		t.Fatalf("expected style BI, got %q", reg.Style)
	}
}

func TestResolveImageSearchDirs_UsesBaseDirImages(t *testing.T) {
	baseDir := filepath.Join("/tmp", "profile")
	dirs := resolveImageSearchDirs(RenderInput{AssetBaseDir: baseDir})
	if len(dirs) != 2 {
		t.Fatalf("expected two image search dirs, got %d", len(dirs))
	}
	want := []string{
		filepath.Join(baseDir, "images"),
		filepath.Join(baseDir, "..", "images"),
	}
	for idx, got := range dirs {
		if got != want[idx] {
			t.Fatalf("unexpected image search dir at index %d: got %q want %q", idx, got, want[idx])
		}
	}
}

func TestResolveI18nInput_AutoDetectsBaseDirI18n(t *testing.T) {
	baseDir := t.TempDir()
	i18nPath := filepath.Join(baseDir, "i18n.json")
	content := `{"_floatSeparator":{"de":","},"_kiloSeparator":{"de":"."},"invoice":{"title":{"de":"Rechnung"}}}`
	if err := os.WriteFile(i18nPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write i18n file: %v", err)
	}

	i18nInst, err := resolveI18nInput("de", RenderInput{AssetBaseDir: baseDir})
	if err != nil {
		t.Fatalf("resolveI18nInput returned error: %v", err)
	}
	if i18nInst.Locale() != "de" {
		t.Fatalf("unexpected locale: got %q want %q", i18nInst.Locale(), "de")
	}
}

func TestResolveI18nInput_ExplicitSourceOverridesBaseDir(t *testing.T) {
	baseDir := t.TempDir()
	i18nPath := filepath.Join(baseDir, "i18n.json")
	baseContent := `{"_floatSeparator":{"de":","},"_kiloSeparator":{"de":"."},"invoice":{"title":{"de":"Basis"}}}`
	if err := os.WriteFile(i18nPath, []byte(baseContent), 0644); err != nil {
		t.Fatalf("failed to write base-dir i18n file: %v", err)
	}

	override := JSONSource{Object: map[string]any{
		"_floatSeparator": map[string]any{"de": ","},
		"_kiloSeparator":  map[string]any{"de": "."},
		"invoice":         map[string]any{"title": map[string]any{"de": "Override"}},
	}}

	i18nInst, err := resolveI18nInput("de", RenderInput{AssetBaseDir: baseDir, I18nSource: override})
	if err != nil {
		t.Fatalf("resolveI18nInput returned error: %v", err)
	}
	if i18nInst.T("invoice.title") != "Override" {
		t.Fatalf("expected explicit i18n override to win")
	}
}

func TestRenderI18nTemplateNode_CanUseSourceAndTemplateFuncs(t *testing.T) {
	fixed := time.Date(2026, 7, 31, 9, 0, 0, 0, time.UTC)
	funcs := DefaultTemplateFuncMapWithContext(FuncContext{
		DefaultLocale:       "en",
		PayloadLocale:       "en",
		DefaultCurrencyCode: "EUR",
		Now: func() time.Time {
			return fixed
		},
	})

	node := map[string]any{
		"invoice": map[string]any{
			"intro": "Month {{formatLocalizedDateOrNow .Source.Invoice.Date \"written-month\"}} for {{.Source.Company.Name}} in {{currency}} ({{currency \"symbol\"}})",
		},
	}

	data := map[string]any{
		"Source": map[string]any{
			"Company": map[string]any{"Name": "ACME"},
			"Invoice": map[string]any{"Date": "2026-07-31"},
		},
	}

	rendered, err := renderI18nTemplateNode(node, funcs, data)
	if err != nil {
		t.Fatalf("renderI18nTemplateNode returned error: %v", err)
	}
	renderedMap, ok := rendered.(map[string]any)
	if !ok {
		t.Fatalf("unexpected rendered type: %T", rendered)
	}
	invoice, ok := renderedMap["invoice"].(map[string]any)
	if !ok {
		t.Fatalf("expected invoice map in rendered i18n data")
	}
	if got, _ := invoice["intro"].(string); got != "Month July for ACME in EUR (€)" {
		t.Fatalf("unexpected rendered i18n value: %q", got)
	}
}

func TestBuildRenderInput_WithI18nTemplateMacrosOption(t *testing.T) {
	input, err := buildRenderInput("out.pdf", WithI18nTemplateMacros(true))
	if err != nil {
		t.Fatalf("buildRenderInput returned error: %v", err)
	}
	if !input.EnableI18nTemplateMacros {
		t.Fatalf("expected EnableI18nTemplateMacros to be true")
	}
}

func TestRenderI18nTemplateNode_FailsOnMissingSourceReference(t *testing.T) {
	node := map[string]any{
		"invoice": map[string]any{
			"subject": "Invoice {{.Source.Invoice.ID}} for {{.Source.Order.ID}}",
		},
	}
	data := map[string]any{
		"Source": map[string]any{
			"Invoice": map[string]any{"ID": "INV-42"},
		},
	}

	_, err := renderI18nTemplateNode(node, nil, data)
	if err == nil {
		t.Fatalf("expected renderI18nTemplateNode to fail on missing source reference")
	}
	if !strings.Contains(err.Error(), ".Source.Order.ID") {
		t.Fatalf("expected error to mention missing source path, got %v", err)
	}
}

func TestRenderToBytes_I18nMacroErrorIncludesFileAndHighlightedToken(t *testing.T) {
	baseDir := t.TempDir()
	html := `
{{define "doc"}}<div id="subject">{{.i18n.invoiceSubject}}</div>{{end}}
{{define "page-number"}}<div>{{.page.pageNumber}}</div>{{end}}`
	css := "@page { size: A4; margin: 20pt; }"
	i18nContent := `{
  "_floatSeparator": {"en": "."},
  "_kiloSeparator": {"en": ","},
  "invoiceSubject": {
    "en": "Invoice {{.Source.Invoice.ID}} for {{.Source.Order.ID}}"
  }
}`
	data := `{"locale":"en","Invoice":{"ID":"INV-42"}}`

	writeFile(t, filepath.Join(baseDir, "doc.html"), html)
	writeFile(t, filepath.Join(baseDir, "doc.css"), css)
	writeFile(t, filepath.Join(baseDir, "i18n.json"), i18nContent)
	writeFile(t, filepath.Join(baseDir, "data.json"), data)

	_, err := RenderToBytes(RenderInput{
		AssetBaseDir:             baseDir,
		DefaultLocale:            "en",
		DefaultCurrencyCode:      "EUR",
		EnableI18nTemplateMacros: true,
	})
	if err == nil {
		t.Fatalf("expected i18n macro expansion to fail")
	}
	errText := err.Error()
	if !strings.Contains(errText, "i18n macro expansion failed") {
		t.Fatalf("expected i18n macro expansion error, got %v", err)
	}
	if !strings.Contains(errText, "i18n.json") {
		t.Fatalf("expected error to include i18n source file path, got %v", err)
	}
	if !strings.Contains(errText, "{{.Source.Order.ID}}") {
		t.Fatalf("expected error to highlight missing token, got %v", err)
	}
}
