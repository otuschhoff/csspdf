package csspdf

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

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

func TestRenderToBytes_LayerOnlyAssetsApplyFlowDefaults(t *testing.T) {
	assets := minimalAssets()
	assets.HTMLLayers = []HTMLLayer{{Name: "document", HTML: assets.HTML}}
	assets.HTML = ""
	assets.Flow = Flow{}

	pdf, err := RenderToBytes(RenderInput{Assets: assets})
	if err != nil {
		t.Fatalf("expected layer-only resolved assets to infer flow defaults: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("expected PDF output")
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

func TestResolveFontRegistrations_FallsBackToParentFonts(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "profile")
	fontsDir := filepath.Join(root, "fonts")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fontsDir, 0755); err != nil {
		t.Fatal(err)
	}
	fontPath := filepath.Join(fontsDir, "ParentFont.ttf")
	if err := os.WriteFile(fontPath, []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	registrations, err := resolveFontRegistrations(RenderInput{AssetBaseDir: baseDir})
	if err != nil {
		t.Fatalf("resolveFontRegistrations returned error: %v", err)
	}
	if len(registrations) != 1 || registrations[0].Family != "ParentFont" || registrations[0].Sources[0] != fontPath {
		t.Fatalf("unexpected parent font registrations: %#v", registrations)
	}
}

func TestResolveFontRegistrations_FallsBackToGrandparentFonts(t *testing.T) {
	root := t.TempDir()
	baseDir := filepath.Join(root, "profiles", "invoice")
	fontsDir := filepath.Join(root, "fonts")
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(fontsDir, 0755); err != nil {
		t.Fatal(err)
	}
	fontPath := filepath.Join(fontsDir, "GrandparentFont.ttf")
	if err := os.WriteFile(fontPath, []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	registrations, err := resolveFontRegistrations(RenderInput{AssetBaseDir: baseDir})
	if err != nil {
		t.Fatalf("resolveFontRegistrations returned error: %v", err)
	}
	if len(registrations) != 1 || registrations[0].Family != "GrandparentFont" || registrations[0].Sources[0] != fontPath {
		t.Fatalf("unexpected grandparent font registrations: %#v", registrations)
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

func TestRenderToBytes_I18nMacroErrorIsStructuredAndRedacted(t *testing.T) {
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
	var diagnosticErr *DiagnosticError
	if !errors.As(err, &diagnosticErr) {
		t.Fatalf("expected structured diagnostic, got %T: %v", err, err)
	}
	if diagnosticErr.Code != DiagnosticTemplate || diagnosticErr.Stage != "i18n-template" || diagnosticErr.Section != "doc" {
		t.Fatalf("unexpected diagnostic provenance: %+v", diagnosticErr)
	}
	if strings.Contains(errText, baseDir) || strings.Contains(errText, "Invoice {{.Source.Invoice.ID}}") || strings.Contains(errText, "\x1b[") {
		t.Fatalf("diagnostic leaked source details or ANSI styling: %q", errText)
	}
}
