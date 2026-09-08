package docflowpdf

import (
	"bytes"
	"context"
	"errors"
	htmltmpl "html/template"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/otuschhoff/csspdf/internal/flowrender"
)

func TestRenderToBytesConfinedRootRendersDefaults(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "doc.html"), `{{define "doc"}}<div>Hello {{.Source.Name}}</div>{{end}}{{define "page-number"}}<div>{{.page.pageNumber}}</div>{{end}}`)
	writeFile(t, filepath.Join(root, "doc.css"), `@page { size: A4; margin: 20pt; }`)
	writeFile(t, filepath.Join(root, "data.json"), `{"Name":"confined"}`)

	pdf, err := RenderToBytes(RenderInput{
		AssetBaseDir:     ".",
		ResourceResolver: ConfinedFileResolver{Root: root},
	})
	if err != nil {
		t.Fatalf("render confined defaults: %v", err)
	}
	if !bytes.HasPrefix(pdf, []byte("%PDF")) {
		t.Fatalf("unexpected output prefix %q", pdf[:4])
	}
}

func TestRenderToBytesConfinedRootRejectsTraversalAndWrongImageType(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(t.TempDir(), "outside.html")
	writeFile(t, outside, `{{define "doc"}}outside{{end}}`)
	resolver := ConfinedFileResolver{Root: root}
	_, err := RenderToBytes(RenderInput{
		AssetInput: &AssetInput{
			HTML:       TextSource{FilePath: "../outside.html"},
			CSS:        TextSource{Text: `@page { margin: 20pt; }`},
			SourceData: JSONSource{Text: `{}`},
		},
		ResourceResolver: resolver,
	})
	if err == nil || !strings.Contains(err.Error(), "must be a local path") {
		t.Fatalf("expected traversal rejection, got %v", err)
	}

	writeFile(t, filepath.Join(root, "not-image.png"), "not an image")
	assets := minimalAssets()
	assets.HTML = `{{define "doc"}}<img src="not-image.png" width="10" height="10">{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`
	_, err = RenderToBytes(RenderInput{Assets: assets, ResourceResolver: resolver})
	if err == nil || !strings.Contains(err.Error(), "not a supported PNG, JPEG, or GIF") {
		t.Fatalf("expected image type rejection, got %v", err)
	}
}

func TestRenderBudgetsRejectSourceTemplateNodesRowsPagesAndOutput(t *testing.T) {
	tests := []struct {
		name  string
		input RenderInput
		stage string
	}{
		{
			name: "source bytes",
			input: RenderInput{AssetInput: &AssetInput{
				HTML: TextSource{Text: `{{define "doc"}}<div>x</div>{{end}}{{define "page-number"}}<div>x</div>{{end}}`},
				CSS:  TextSource{Text: `@page { margin: 10pt; }`}, SourceData: JSONSource{Text: `{}`},
			}, Limits: RenderLimits{SourceBytes: 20}},
			stage: "source bytes",
		},
		{
			name: "template output",
			input: func() RenderInput {
				a := minimalAssets()
				a.HTML = `{{define "doc"}}<div>` + strings.Repeat("x", 200) + `</div>{{end}}{{define "page-number"}}<div>x</div>{{end}}`
				return RenderInput{Assets: a, Limits: RenderLimits{TemplateOutputBytes: 32}}
			}(),
			stage: "template output bytes",
		},
		{
			name: "nodes",
			input: func() RenderInput {
				a := minimalAssets()
				return RenderInput{Assets: a, Limits: RenderLimits{Nodes: 1}}
			}(),
			stage: "PDFDOM nodes",
		},
		{
			name: "rows",
			input: func() RenderInput {
				a := minimalAssets()
				a.HTML = `{{define "doc"}}<table width="200" padding="2" row-height-min="10"><tr><td>a</td></tr><tr><td>b</td></tr></table>{{end}}{{define "page-number"}}<div>x</div>{{end}}`
				return RenderInput{Assets: a, Limits: RenderLimits{Rows: 1}}
			}(),
			stage: "table rows",
		},
		{
			name: "pages",
			input: func() RenderInput {
				a := minimalAssets()
				a.HTML = `{{define "doc"}}<div>` + strings.Repeat("line\n", 100) + `</div>{{end}}{{define "page-number"}}<div>x</div>{{end}}`
				return RenderInput{Assets: a, PageWidth: 200, PageHeight: 100, DefaultMargins: pageMargins(10), Limits: RenderLimits{Pages: 1}}
			}(),
			stage: "pages budget exceeded",
		},
		{
			name: "output bytes",
			input: func() RenderInput {
				a := minimalAssets()
				return RenderInput{Assets: a, Limits: RenderLimits{OutputBytes: 100}}
			}(),
			stage: "PDF output bytes",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := RenderToBytes(test.input)
			if err == nil || !strings.Contains(err.Error(), test.stage) {
				t.Fatalf("expected %q budget error, got %v", test.stage, err)
			}
			var budgetErr *BudgetError
			if !errors.As(err, &budgetErr) {
				t.Fatalf("expected typed BudgetError, got %T: %v", err, err)
			}
			if !errors.Is(err, ErrLimitExceeded) {
				t.Fatalf("expected ErrLimitExceeded, got %T: %v", err, err)
			}
		})
	}
}

func TestRenderBudgetsRejectResolvedAssetsAndSourceData(t *testing.T) {
	assets := minimalAssets()
	_, err := RenderToBytes(RenderInput{Assets: assets, Limits: RenderLimits{SourceBytes: 20}})
	assertBudgetStage(t, err, "template HTML source bytes")

	assets = minimalAssets()
	assets.SourceData = map[string]any{"value": strings.Repeat("x", 500)}
	_, err = RenderToBytes(RenderInput{Assets: assets, Limits: RenderLimits{SourceBytes: 150}})
	assertBudgetStage(t, err, "source data bytes")

	assets = minimalAssets()
	assets.HTML = ""
	assets.HTMLLayers = []HTMLLayer{{Name: "one", HTML: strings.Repeat("a", 80)}, {Name: "two", HTML: strings.Repeat("b", 80)}}
	_, err = RenderToBytes(RenderInput{Assets: assets, Limits: RenderLimits{SourceBytes: 150}})
	assertBudgetStage(t, err, "template HTML source bytes")
}

func TestI18nTemplateMacrosShareOutputBudgetAndContext(t *testing.T) {
	_, err := renderI18nTemplateNodeWithOptions(map[string]any{"one": "1234", "two": "5678"}, nil, nil, context.Background(), 7)
	assertBudgetStage(t, err, "i18n template output bytes")

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = renderI18nTemplateNodeWithOptions(map[string]any{"value": "text"}, nil, nil, ctx, 100)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected canceled i18n expansion, got %v", err)
	}
}

func TestRenderBudgetsRejectImageBytesAndPixels(t *testing.T) {
	root := t.TempDir()
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "image.png"), encoded.String())
	assets := minimalAssets()
	assets.HTML = `{{define "doc"}}<img src="image.png" width="2" height="2">{{end}}{{define "page-number"}}<div>x</div>{{end}}`
	input := RenderInput{Assets: assets, ResourceResolver: ConfinedFileResolver{Root: root}}

	input.Limits = RenderLimits{ImageBytes: 8}
	_, err := RenderToBytes(input)
	var resourceLimit *LimitError
	if !errors.As(err, &resourceLimit) {
		t.Fatalf("expected typed image byte limit error, got %T: %v", err, err)
	}

	input.Limits = RenderLimits{ImagePixels: 3}
	_, err = RenderToBytes(input)
	assertBudgetStage(t, err, "decoded image pixels")
}

func TestPageNumberTemplateAppliesComplexityBudget(t *testing.T) {
	assets := minimalAssets()
	assets.HTML = `{{define "doc"}}<div>x</div>{{end}}{{define "page-number"}}<table><tr><td>a</td></tr><tr><td>b</td></tr></table>{{end}}`
	limits, err := normalizeRenderLimits(RenderLimits{Rows: 1})
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := buildArtifactContext(context.Background(), RenderInput{Assets: assets}, DefaultRenderLimits())
	if err != nil {
		t.Fatalf("build artifact: %v", err)
	}
	_, err = pageNumberTemplateFlowElements(context.Background(), limits, &flowrender.ComplexityBudget{}, artifact.layout, assets, assets.SourceData, 2, 2, RenderInput{Assets: assets})
	assertBudgetStage(t, err, "table rows")
}

func TestPageNumberTemplateSharesMainFlowComplexityBudget(t *testing.T) {
	assets := minimalAssets()
	assets.HTML = `{{define "doc"}}<table width="200" padding="2" row-height-min="10"><tr><td>main</td></tr></table>{{end}}{{define "page-number"}}<table width="200" padding="2" row-height-min="10"><tr><td>page</td></tr></table>{{end}}`
	limits, err := normalizeRenderLimits(RenderLimits{Rows: 1})
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := buildArtifactContext(context.Background(), RenderInput{Assets: minimalAssets()}, DefaultRenderLimits())
	if err != nil {
		t.Fatalf("build artifact: %v", err)
	}
	complexity := &flowrender.ComplexityBudget{}
	if err := renderMainFlow(context.Background(), limits, complexity, artifact.layout, assets, assets.SourceData, RenderInput{Assets: assets}); err != nil {
		t.Fatalf("main flow should consume exactly one row: %v", err)
	}
	_, err = pageNumberTemplateFlowElements(context.Background(), limits, complexity, artifact.layout, assets, assets.SourceData, 2, 2, RenderInput{Assets: assets})
	assertBudgetStage(t, err, "table rows")
}

func TestConfinedRenderRejectsInvalidFontContent(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "fake.ttf"), "not a font")
	_, err := RenderToBytes(RenderInput{
		Assets:            minimalAssets(),
		ResourceResolver:  ConfinedFileResolver{Root: root},
		FontRegistrations: []FontRegistration{{Family: "Fake", Sources: []string{"fake.ttf"}}},
	})
	if err == nil || !strings.Contains(err.Error(), "not a supported TTF, OTF, or TTC") {
		t.Fatalf("expected invalid font rejection, got %v", err)
	}
}

func TestEmitArtifactContextCancelsDuringOutput(t *testing.T) {
	limits, err := normalizeRenderLimits(RenderLimits{})
	if err != nil {
		t.Fatal(err)
	}
	assets := minimalAssets()
	assets.HTML = `{{define "doc"}}<div>` + strings.Repeat("output ", 1000) + `</div>{{end}}{{define "page-number"}}<div>x</div>{{end}}`
	artifact, err := buildArtifactContext(context.Background(), RenderInput{Assets: assets}, limits)
	if err != nil {
		t.Fatalf("build artifact: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	writer := &cancelingWriter{cancel: cancel}
	err = emitArtifactContext(ctx, artifact, writer, limits.OutputBytes)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation during emission, got %v", err)
	}
}

type cancelingWriter struct {
	cancel context.CancelFunc
	writes int
}

func (w *cancelingWriter) Write(data []byte) (int, error) {
	w.writes++
	if w.writes == 1 {
		w.cancel()
	}
	return len(data), nil
}

func assertBudgetStage(t *testing.T, err error, stage string) {
	t.Helper()
	var budgetErr *BudgetError
	if !errors.As(err, &budgetErr) || budgetErr.Stage != stage {
		t.Fatalf("expected %q BudgetError, got %T: %v", stage, err, err)
	}
}

func TestRenderToFileBudgetFailurePreservesDestination(t *testing.T) {
	output := filepath.Join(t.TempDir(), "existing.pdf")
	if err := os.WriteFile(output, []byte("ORIGINAL"), 0o600); err != nil {
		t.Fatal(err)
	}
	input := RenderInput{Assets: minimalAssets(), Limits: RenderLimits{OutputBytes: 100}}
	if err := RenderToFile(input, output); err == nil {
		t.Fatal("expected output budget failure")
	}
	data, err := os.ReadFile(output)
	if err != nil || string(data) != "ORIGINAL" {
		t.Fatalf("destination after failure = %q, %v", data, err)
	}
}

func TestRenderContextCancellationBeforePreparationAndDuringExpansion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := RenderToBytesContext(ctx, RenderInput{Assets: minimalAssets()})
	if !errors.Is(err, context.Canceled) || !strings.Contains(err.Error(), "before preparation") {
		t.Fatalf("expected preparation cancellation, got %v", err)
	}

	ctx, cancel = context.WithCancel(context.Background())
	assets := minimalAssets()
	assets.HTML = `{{define "doc"}}<div>{{cancel}}</div>{{end}}{{define "page-number"}}<div>x</div>{{end}}`
	_, err = RenderToBytesContext(ctx, RenderInput{
		Assets: assets,
		FuncMapFactoryEx: func(FuncContext) htmltmpl.FuncMap {
			return htmltmpl.FuncMap{"cancel": func() string { cancel(); return "stopped" }}
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected expansion cancellation, got %v", err)
	}
}

func TestPartialRenderDoesNotSuppressBudgetsOrCancellation(t *testing.T) {
	assets := minimalAssets()
	assets.HTML = `{{define "doc"}}<div>` + strings.Repeat("x", 200) + `</div>{{end}}{{define "page-number"}}<div>x</div>{{end}}`
	_, err := RenderToBytes(RenderInput{Assets: assets, AllowPartialRender: true, Limits: RenderLimits{TemplateOutputBytes: 32}})
	assertBudgetStage(t, err, "template output bytes")

	ctx, cancel := context.WithCancel(context.Background())
	assets = minimalAssets()
	assets.HTML = `{{define "doc"}}<div>{{cancel}}</div>{{end}}{{define "page-number"}}<div>x</div>{{end}}`
	_, err = RenderToBytesContext(ctx, RenderInput{
		Assets: assets, AllowPartialRender: true,
		FuncMapFactoryEx: func(FuncContext) htmltmpl.FuncMap {
			return htmltmpl.FuncMap{"cancel": func() string { cancel(); return "stopped" }}
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected partial render to preserve cancellation, got %v", err)
	}
}

func TestOperationalBoundaryErrorsAreNeverRecoverable(t *testing.T) {
	if !isOperationalBoundaryError(&BudgetError{Stage: "test", Limit: 1}) {
		t.Fatal("budget error must be non-recoverable")
	}
	if !isOperationalBoundaryError(context.Canceled) || !isOperationalBoundaryError(context.DeadlineExceeded) {
		t.Fatal("context termination must be non-recoverable")
	}
	if isOperationalBoundaryError(errors.New("ordinary render error")) {
		t.Fatal("ordinary render errors may follow partial-render policy")
	}
}

func TestEmitArtifactContextRejectsCancellationBeforeOutput(t *testing.T) {
	limits, err := normalizeRenderLimits(RenderLimits{})
	if err != nil {
		t.Fatal(err)
	}
	artifact, err := buildArtifactContext(context.Background(), RenderInput{Assets: minimalAssets()}, limits)
	if err != nil {
		t.Fatalf("build artifact: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	var output bytes.Buffer
	err = emitArtifactContext(ctx, artifact, &output, limits.OutputBytes)
	if !errors.Is(err, context.Canceled) || output.Len() != 0 {
		t.Fatalf("emission cancellation = %v, output bytes=%d", err, output.Len())
	}
}

func pageMargins(value float64) PageMargins {
	return PageMargins{Top: value, Right: value, Bottom: value, Left: value}
}
