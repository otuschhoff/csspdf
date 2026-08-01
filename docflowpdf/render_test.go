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
		PageCount:           1,
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
		WithPageCount(1),
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
		PageCount:           1,
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
	assets.Flow.PageNumber.Template = "missing-page-number-template"

	logger := &testLogger{}
	var out bytes.Buffer
	err := RenderToWriter(RenderInput{
		Assets:              assets,
		PageCount:           2,
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
		Logger:              logger,
	}, &out)
	if err != nil {
		t.Fatalf("RenderToWriter returned error: %v", err)
	}
	if len(logger.warnings) == 0 {
		t.Fatalf("expected warning log for page-number render issue")
	}
}

func TestRender_UsesContextFuncFactoryNow(t *testing.T) {
	assets := minimalAssets()
	assets.HTML = `{{define "doc"}}<div>{{fmtNow}}</div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`
	fixed := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	called := false
	_, err := RenderToBytes(RenderInput{
		Assets:              assets,
		PageCount:           1,
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
	err := RenderToFile(RenderInput{Assets: minimalAssets(), PageCount: 1}, "")
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
		PageCount:           1,
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
		PageCount:           1,
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
		PageCount:           1,
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
		PageCount:           1,
		DefaultLocale:       "xy",
		DefaultCurrencyCode: "EUR",
		I18nSource:          JSONSource{FS: fs, FSPath: "cfg/i18n.json"},
	})
	if err != nil {
		t.Fatalf("expected render to succeed with fs i18n source: %v", err)
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

func TestResolveImageSearchDirs_UsesBaseDirImages(t *testing.T) {
	baseDir := filepath.Join("/tmp", "profile")
	dirs := resolveImageSearchDirs(RenderInput{AssetBaseDir: baseDir})
	if len(dirs) != 1 {
		t.Fatalf("expected one image search dir, got %d", len(dirs))
	}
	want := filepath.Join(baseDir, "images")
	if dirs[0] != want {
		t.Fatalf("unexpected image search dir: got %q want %q", dirs[0], want)
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
