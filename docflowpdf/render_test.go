package docflowpdf

import (
	"bytes"
	"errors"
	"fmt"
	htmltmpl "html/template"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
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

func TestRenderToBytes_FixedClockIsProcessDeterministic(t *testing.T) {
	fixedNow := func() time.Time { return time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC) }
	input := RenderInput{Assets: minimalAssets(), Now: fixedNow}
	if outputPath := os.Getenv("CSSPDF_DETERMINISM_OUTPUT"); outputPath != "" {
		pdf, err := RenderToBytes(input)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(outputPath, pdf, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}

	first, err := RenderToBytes(input)
	if err != nil {
		t.Fatal(err)
	}
	second, err := RenderToBytes(input)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("fixed-clock renders differ within one process")
	}

	paths := []string{filepath.Join(t.TempDir(), "first.pdf"), filepath.Join(t.TempDir(), "second.pdf")}
	for _, outputPath := range paths {
		command := exec.Command(os.Args[0], "-test.run=^TestRenderToBytes_FixedClockIsProcessDeterministic$")
		command.Env = append(os.Environ(), "CSSPDF_DETERMINISM_OUTPUT="+outputPath)
		if output, err := command.CombinedOutput(); err != nil {
			t.Fatalf("determinism subprocess failed: %v\n%s", err, output)
		}
	}
	processFirst, err := os.ReadFile(paths[0])
	if err != nil {
		t.Fatal(err)
	}
	processSecond, err := os.ReadFile(paths[1])
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(processFirst, processSecond) || !bytes.Equal(first, processFirst) {
		t.Fatal("fixed-clock renders differ across processes")
	}
}

func TestRenderToBytes_ComplexLayoutsAreDeterministic(t *testing.T) {
	fixedNow := func() time.Time { return time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC) }
	workloads := map[string]RenderInput{
		"layered-multi-section": benchmarkLayeredDocument(fixedNow),
		"long-table":            benchmarkLongTable(fixedNow, 50),
	}
	for name, input := range workloads {
		t.Run(name, func(t *testing.T) {
			var expected []byte
			for iteration := 0; iteration < 3; iteration++ {
				pdf, err := RenderToBytes(input)
				if err != nil {
					t.Fatal(err)
				}
				if iteration == 0 {
					expected = pdf
					continue
				}
				if !bytes.Equal(expected, pdf) {
					offset := firstDifferentByte(expected, pdf)
					t.Fatalf("render %d differs from the first fixed-clock render at byte %d (lengths %d and %d)", iteration+1, offset, len(expected), len(pdf))
				}
			}
		})
	}
}

func firstDifferentByte(left, right []byte) int {
	limit := min(len(left), len(right))
	for index := 0; index < limit; index++ {
		if left[index] != right[index] {
			return index
		}
	}
	return limit
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

func TestResolvePageDimensions_RejectsNonFiniteOverrides(t *testing.T) {
	testCases := []struct {
		name  string
		input RenderInput
	}{
		{name: "NaN width", input: RenderInput{PageWidth: math.NaN()}},
		{name: "positive infinite width", input: RenderInput{PageWidth: math.Inf(1)}},
		{name: "negative infinite height", input: RenderInput{PageHeight: math.Inf(-1)}},
	}
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if _, _, err := resolvePageDimensions(testCase.input); err == nil || !strings.Contains(err.Error(), "finite") {
				t.Fatalf("expected finite-dimension error, got %v", err)
			}
		})
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

func TestRenderToBytes_DefaultI18nIsIndependentOfWorkingDirectory(t *testing.T) {
	originalDir, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(t.TempDir()); err != nil {
		t.Fatalf("change to empty working directory: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(originalDir); err != nil {
			t.Errorf("restore working directory: %v", err)
		}
	})

	pdf, err := RenderToBytes(RenderInput{
		Assets:              minimalAssets(),
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
	})
	if err != nil {
		t.Fatalf("RenderToBytes returned error in empty working directory: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("expected PDF output")
	}
}

func TestRenderToBytes_CustomHelperReceivesExactLargeInteger(t *testing.T) {
	assets := minimalAssets()
	assets.HTML = `
{{define "doc"}}<div>{{capture .Source.id}}</div>{{end}}
{{define "page-number"}}<div>{{.Page}}/{{.Total}}</div>{{end}}`
	const want = "9007199254740993"
	var captured string

	_, err := RenderToBytes(RenderInput{
		Assets:     assets,
		SourceData: map[string]any{"id": int64(9007199254740993), "locale": "en"},
		FuncMapFactoryEx: func(FuncContext) htmltmpl.FuncMap {
			return htmltmpl.FuncMap{"capture": func(value any) string {
				captured = fmt.Sprint(value)
				return captured
			}}
		},
	})
	if err != nil {
		t.Fatalf("RenderToBytes returned error: %v", err)
	}
	if captured != want {
		t.Fatalf("custom helper received %q, want %q", captured, want)
	}
}

func TestRenderToBytes_ConcurrentReuseDoesNotMutateInput(t *testing.T) {
	assets := minimalAssets()
	assets.Flow.MainFlow[0].Transformer = ""
	assets.Flow.PageNumber.Transformer = ""
	originalFlow := cloneFlow(assets.Flow)
	input := RenderInput{Assets: assets}

	const workers = 8
	errorsByWorker := make(chan error, workers)
	var wait sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := RenderToBytes(input)
			errorsByWorker <- err
		}()
	}
	wait.Wait()
	close(errorsByWorker)
	for err := range errorsByWorker {
		if err != nil {
			t.Fatalf("concurrent RenderToBytes returned error: %v", err)
		}
	}
	if !reflect.DeepEqual(input.Assets.Flow, originalFlow) {
		t.Fatalf("shared input flow was mutated: got %+v want %+v", input.Assets.Flow, originalFlow)
	}
}

type testLogger struct {
	warnings []string
}

func (l *testLogger) Warnf(format string, args ...any) {
	l.warnings = append(l.warnings, fmt.Sprintf(format, args...))
}

func TestRender_InvalidSpanAttributeFailsStrictAndWarnsInLegacyMode(t *testing.T) {
	assets := minimalAssets()
	assets.HTML = `
{{define "doc"}}<div><span font-size="large">text</span></div>{{end}}
{{define "page-number"}}<div>{{.Page}}/{{.Total}}</div>{{end}}`

	_, err := RenderToBytes(RenderInput{Assets: assets})
	var diagnosticErr *DiagnosticError
	if !errors.As(err, &diagnosticErr) || diagnosticErr.Code != DiagnosticTemplate {
		t.Fatalf("expected template diagnostic for invalid span attribute, got %T: %v", err, err)
	}
	if !strings.Contains(err.Error(), `font-size=&#34;large&#34;`) && !strings.Contains(err.Error(), `font-size="large"`) {
		t.Fatalf("strict error does not identify invalid attribute: %v", err)
	}

	logger := &testLogger{}
	_, err = RenderToBytes(RenderInput{Assets: assets, AllowPartialRender: true, Logger: logger})
	if err != nil {
		t.Fatalf("legacy render returned error: %v", err)
	}
	found := false
	for _, warning := range logger.warnings {
		found = found || strings.Contains(warning, `invalid span attribute font-size="large"`)
	}
	if !found {
		t.Fatalf("unexpected legacy warnings: %q", logger.warnings)
	}
}

func TestRender_LegacyPartialRenderingUsesLoggerForRecoverableErrors(t *testing.T) {
	assets := minimalAssets()
	assets.Flow.MainFlow[0].Template = "missing-doc-template"

	logger := &testLogger{}
	var out bytes.Buffer
	err := RenderToWriter(RenderInput{
		Assets:              assets,
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
		Logger:              logger,
		AllowPartialRender:  true,
	}, &out)
	if err != nil {
		t.Fatalf("RenderToWriter returned error: %v", err)
	}
	if len(logger.warnings) == 0 {
		t.Fatalf("expected warning log for main-flow render issue")
	}
}

func TestRenderToBytes_MissingTemplateFailsForLegacyAndLayeredHTML(t *testing.T) {
	testCases := []struct {
		name   string
		assets Assets
	}{
		{name: "legacy", assets: minimalAssets()},
		{name: "layered", assets: func() Assets {
			assets := minimalAssets()
			assets.HTMLLayers = []HTMLLayer{{Name: "base", HTML: assets.HTML}}
			assets.HTML = ""
			return assets
		}()},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			assets := testCase.assets
			assets.Flow.MainFlow[0].Template = "missing-doc-template"
			_, err := RenderToBytes(RenderInput{Assets: assets})
			if err == nil || !strings.Contains(err.Error(), "missing-doc-template") {
				t.Fatalf("expected missing template error, got %v", err)
			}
		})
	}
}

func TestRenderToBytes_MissingRequiredTemplateValueFails(t *testing.T) {
	assets := minimalAssets()
	assets.HTML = `
{{define "doc"}}<div>{{.Source.RequiredValue}}</div>{{end}}
{{define "page-number"}}<div>{{.Page}}/{{.Total}}</div>{{end}}`

	_, err := RenderToBytes(RenderInput{Assets: assets})
	if err == nil || !strings.Contains(err.Error(), "RequiredValue") {
		t.Fatalf("expected missing required value error, got %v", err)
	}
}

func TestRenderToBytes_MissingImageFails(t *testing.T) {
	assets := minimalAssets()
	missingPath := filepath.Join(t.TempDir(), "missing.png")
	assets.HTML = fmt.Sprintf(`
{{define "doc"}}<img src=%q width="10" height="10">{{end}}
{{define "page-number"}}<div>{{.Page}}/{{.Total}}</div>{{end}}`, missingPath)

	_, err := RenderToBytes(RenderInput{Assets: assets})
	if err == nil || !strings.Contains(err.Error(), "image not found") {
		t.Fatalf("expected missing image error, got %v", err)
	}
}

func TestRenderToBytes_HTMLLayers_MissingNestedContentTemplateFails(t *testing.T) {
	assetInput := AssetInput{
		HTML: TextSource{Text: `
{{define "page-number"}}<div>{{.page.pageNumber}}</div>{{end}}`},
		HTMLLayers: []HTMLLayerInput{{
			Name: "wrapper",
			Source: TextSource{Text: `
{{define "doc"}}<div>{{template "default-letterhead" .}}{{template "document-content" .}}</div>{{end}}
{{define "default-letterhead"}}<div>LH</div>{{end}}`},
		}},
		CSS:        TextSource{Text: "@page { size: A4; margin: 20pt; }"},
		Flow:       JSONSource{Object: Flow{MainFlow: []Section{{Template: "doc", Transformer: "generic"}}, PageNumber: Section{Template: "page-number", Transformer: "generic"}}},
		SourceData: JSONSource{Object: map[string]any{"Name": "Docflow"}},
	}

	_, err := RenderToBytes(RenderInput{
		AssetInput:          &assetInput,
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
		FuncMapFactoryEx:    DefaultTemplateFuncMapWithContext,
	})
	if err == nil {
		t.Fatalf("expected render to fail when wrapper references missing document-content template")
	}
	if !strings.Contains(err.Error(), `no such template "document-content"`) {
		t.Fatalf("unexpected error for missing nested content template: %v", err)
	}
}

func TestRenderToBytes_HTMLLayers_InvalidTemplateSyntaxFails(t *testing.T) {
	assetInput := AssetInput{
		HTML: TextSource{Text: `
{{define "document-content"}}<div>Body</div>{{end}}
{{define "page-number"}}<div>{{.page.pageNumber}}</div>{{end}}`},
		HTMLLayers: []HTMLLayerInput{{
			Name: "wrapper",
			Source: TextSource{Text: `
{{define "doc"}}<div>{{template "document-content" .}}</div>`},
		}},
		CSS:  TextSource{Text: "@page { size: A4; margin: 20pt; }"},
		Flow: JSONSource{Object: Flow{MainFlow: []Section{{Template: "doc", Transformer: "generic"}}, PageNumber: Section{Template: "page-number", Transformer: "generic"}}},
	}

	_, err := RenderToBytes(RenderInput{
		AssetInput:          &assetInput,
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
	})
	if err == nil {
		t.Fatalf("expected render to fail for invalid wrapper template syntax")
	}
	if !strings.Contains(err.Error(), "failed to parse template source") {
		t.Fatalf("unexpected invalid-template error: %v", err)
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
