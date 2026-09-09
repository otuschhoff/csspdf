package csspdf

import (
	"bytes"
	"errors"
	htmltmpl "html/template"
	"os"
	"path/filepath"
	"runtime"
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

func TestRenderToFile_PreservesExistingDestinationOnRenderFailure(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "document.pdf")
	original := []byte("existing output")
	if err := os.WriteFile(outputPath, original, 0640); err != nil {
		t.Fatalf("write existing destination: %v", err)
	}

	err := RenderToFile(RenderInput{Assets: Assets{}}, outputPath)
	if err == nil {
		t.Fatalf("expected render failure")
	}
	got, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("read existing destination: %v", readErr)
	}
	if !bytes.Equal(got, original) {
		t.Fatalf("destination changed after render failure: got %q", got)
	}
	assertNoAtomicOutputTemps(t, dir)
}

func TestRenderToFile_AtomicallyReplacesDestinationAndPreservesMode(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "document.pdf")
	if err := os.WriteFile(outputPath, []byte("old"), 0640); err != nil {
		t.Fatalf("write existing destination: %v", err)
	}

	err := RenderToFile(RenderInput{
		Assets:              minimalAssets(),
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
	}, outputPath)
	if err != nil {
		t.Fatalf("RenderToFile returned error: %v", err)
	}
	got, readErr := os.ReadFile(outputPath)
	if readErr != nil {
		t.Fatalf("read replaced destination: %v", readErr)
	}
	if !bytes.HasPrefix(got, []byte("%PDF")) {
		t.Fatalf("expected PDF output, got %q", got)
	}
	info, statErr := os.Stat(outputPath)
	if statErr != nil {
		t.Fatalf("stat replaced destination: %v", statErr)
	}
	if runtime.GOOS != "windows" {
		if gotMode := info.Mode().Perm(); gotMode != 0640 {
			t.Fatalf("destination mode = %o, want 640", gotMode)
		}
	}
	assertNoAtomicOutputTemps(t, dir)
}

func TestRenderToFile_NewDestinationIsOwnerOnly(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows does not expose Unix permission bits")
	}
	outputPath := filepath.Join(t.TempDir(), "document.pdf")
	err := RenderToFile(RenderInput{Assets: minimalAssets()}, outputPath)
	if err != nil {
		t.Fatalf("RenderToFile returned error: %v", err)
	}
	info, statErr := os.Stat(outputPath)
	if statErr != nil {
		t.Fatalf("stat output: %v", statErr)
	}
	if got := info.Mode().Perm(); got != 0600 {
		t.Fatalf("new output mode = %o, want 600", got)
	}
}

func assertNoAtomicOutputTemps(t *testing.T, dir string) {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(dir, ".document.pdf.tmp-*"))
	if err != nil {
		t.Fatalf("glob temporary outputs: %v", err)
	}
	if len(matches) != 0 {
		t.Fatalf("temporary outputs were not cleaned up: %v", matches)
	}
}

type failingAtomicOutput struct {
	name       string
	writeErr   error
	closeErr   error
	closeCalls int
}

func (f *failingAtomicOutput) Write(data []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(data), nil
}

func (f *failingAtomicOutput) Chmod(os.FileMode) error { return nil }
func (f *failingAtomicOutput) Name() string            { return f.name }
func (f *failingAtomicOutput) Close() error {
	f.closeCalls++
	return f.closeErr
}

func TestWriteFileAtomically_PreservesDestinationAndCleansUpOnFailures(t *testing.T) {
	testCases := []struct {
		name      string
		writeErr  error
		closeErr  error
		renameErr error
		wantError string
	}{
		{name: "write", writeErr: errors.New("disk full"), wantError: "failed to write temporary PDF"},
		{name: "close", closeErr: errors.New("flush failed"), wantError: "failed to close temporary PDF"},
		{name: "rename", renameErr: errors.New("replace failed"), wantError: "failed to replace output file"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			dir := t.TempDir()
			outputPath := filepath.Join(dir, "document.pdf")
			original := []byte("existing output")
			if err := os.WriteFile(outputPath, original, 0600); err != nil {
				t.Fatalf("write existing destination: %v", err)
			}

			removed := false
			temp := &failingAtomicOutput{
				name:     filepath.Join(dir, ".document.pdf.tmp-test"),
				writeErr: testCase.writeErr,
				closeErr: testCase.closeErr,
			}
			err := writeFileAtomically(outputPath, []byte("new output"), atomicOutputOps{
				createTemp: func(string, string) (atomicOutputFile, error) { return temp, nil },
				stat:       os.Stat,
				rename: func(string, string) error {
					return testCase.renameErr
				},
				remove: func(string) error {
					removed = true
					return nil
				},
			})
			if err == nil || !strings.Contains(err.Error(), testCase.wantError) {
				t.Fatalf("error = %v, want containing %q", err, testCase.wantError)
			}
			got, readErr := os.ReadFile(outputPath)
			if readErr != nil {
				t.Fatalf("read existing destination: %v", readErr)
			}
			if !bytes.Equal(got, original) {
				t.Fatalf("destination changed after %s failure: got %q", testCase.name, got)
			}
			if !removed {
				t.Fatalf("temporary output was not removed after %s failure", testCase.name)
			}
			if temp.closeCalls != 1 {
				t.Fatalf("close calls = %d, want 1", temp.closeCalls)
			}
		})
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
