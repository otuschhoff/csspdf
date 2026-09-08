package docflowpdf

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"testing"
	"time"

	htmltmpl "html/template"
)

type optionTestLogger struct{}

func (*optionTestLogger) Warnf(string, ...any) {}

func TestRenderOptionsSetExpectedFields(t *testing.T) {
	now := func() time.Time { return time.Date(2026, time.September, 8, 12, 0, 0, 0, time.UTC) }
	legacyFactory := func(string, string) htmltmpl.FuncMap {
		return htmltmpl.FuncMap{"legacy": func() string { return "ok" }}
	}
	contextFactory := func(FuncContext) htmltmpl.FuncMap { return htmltmpl.FuncMap{"context": func() string { return "ok" }} }
	logger := &optionTestLogger{}
	warningWriter := &bytes.Buffer{}
	assets := Assets{HTML: "html"}
	assetInput := AssetInput{HTML: TextSource{Text: "html"}}
	source := map[string]any{"id": "source"}
	i18nSource := JSONSource{Text: `{"hello":"world"}`}
	limits := RenderLimits{Pages: 17}
	margins := PageMargins{Top: 1, Right: 2, Bottom: 3, Left: 4}
	fonts := []FontRegistration{{Family: "Example", Style: "B", Sources: []string{"font.ttf"}}}

	tests := []struct {
		name   string
		option RenderOption
		verify func(*testing.T, RenderInput)
	}{
		{"asset base directory", WithAssetBaseDir("assets"), func(t *testing.T, input RenderInput) { assertEqual(t, input.AssetBaseDir, "assets") }},
		{"confined resource root", WithConfinedResourceRoot("root"), func(t *testing.T, input RenderInput) {
			assertEqual(t, input.AssetBaseDir, ".")
			assertEqual(t, input.ResourceResolver, ConfinedFileResolver{Root: "root"})
		}},
		{"trusted file access", WithTrustedFileAccess(), func(t *testing.T, input RenderInput) { assertEqual(t, input.ResourceResolver, TrustedFileResolver{}) }},
		{"render limits", WithRenderLimits(limits), func(t *testing.T, input RenderInput) { assertEqual(t, input.Limits, limits) }},
		{"resolved assets", WithAssets(assets), func(t *testing.T, input RenderInput) {
			assertEqual(t, input.Assets, assets)
			if input.AssetInput != nil {
				t.Fatal("WithAssets did not clear AssetInput")
			}
		}},
		{"asset input", WithAssetInput(assetInput), func(t *testing.T, input RenderInput) {
			if input.AssetInput == nil || !reflect.DeepEqual(*input.AssetInput, assetInput) {
				t.Fatalf("AssetInput = %#v, want %#v", input.AssetInput, assetInput)
			}
		}},
		{"source data", WithSourceData(source), func(t *testing.T, input RenderInput) { assertEqual(t, input.SourceData, source) }},
		{"i18n source", WithI18nSource(i18nSource), func(t *testing.T, input RenderInput) { assertEqual(t, input.I18nSource, i18nSource) }},
		{"i18n macros", WithI18nTemplateMacros(true), func(t *testing.T, input RenderInput) { assertEqual(t, input.EnableI18nTemplateMacros, true) }},
		{"legacy partial rendering", WithLegacyPartialRendering(true), func(t *testing.T, input RenderInput) { assertEqual(t, input.AllowPartialRender, true) }},
		{"font registrations", WithFontRegistrations(fonts...), func(t *testing.T, input RenderInput) { assertEqual(t, input.FontRegistrations, fonts) }},
		{"page size", WithPageSize(612, 792), func(t *testing.T, input RenderInput) {
			assertEqual(t, input.PageWidth, 612.0)
			assertEqual(t, input.PageHeight, 792.0)
		}},
		{"page format", WithPageFormat("letter"), func(t *testing.T, input RenderInput) { assertEqual(t, input.PageFormat, "letter") }},
		{"page orientation", WithPageOrientation("landscape"), func(t *testing.T, input RenderInput) { assertEqual(t, input.PageOrientation, "landscape") }},
		{"page width", WithPageWidth(612), func(t *testing.T, input RenderInput) { assertEqual(t, input.PageWidth, 612.0) }},
		{"page height", WithPageHeight(792), func(t *testing.T, input RenderInput) { assertEqual(t, input.PageHeight, 792.0) }},
		{"default locale", WithDefaultLocale("de"), func(t *testing.T, input RenderInput) { assertEqual(t, input.DefaultLocale, "de") }},
		{"default currency", WithDefaultCurrencyCode("CHF"), func(t *testing.T, input RenderInput) { assertEqual(t, input.DefaultCurrencyCode, "CHF") }},
		{"default margins", WithDefaultMargins(margins), func(t *testing.T, input RenderInput) { assertEqual(t, input.DefaultMargins, margins) }},
		{"horizontal margins", WithPageMarginsLeftRight(23), func(t *testing.T, input RenderInput) {
			assertEqual(t, input.DefaultMargins.Left, 23.0)
			assertEqual(t, input.DefaultMargins.Right, 23.0)
		}},
		{"vertical margins", WithPageMarginsTopBottom(31), func(t *testing.T, input RenderInput) {
			assertEqual(t, input.DefaultMargins.Top, 31.0)
			assertEqual(t, input.DefaultMargins.Bottom, 31.0)
		}},
		{"clock", WithNow(now), func(t *testing.T, input RenderInput) { assertEqual(t, input.Now(), now()) }},
		{"context function factory", WithFuncMapFactoryEx(contextFactory), func(t *testing.T, input RenderInput) {
			if _, ok := input.FuncMapFactoryEx(FuncContext{})["context"]; !ok {
				t.Fatal("context function factory was not retained")
			}
		}},
		{"legacy function factory", WithFuncMapFactory(legacyFactory), func(t *testing.T, input RenderInput) {
			if _, ok := input.FuncMapFactory("en", "de")["legacy"]; !ok {
				t.Fatal("legacy function factory was not retained")
			}
		}},
		{"logger", WithLogger(logger), func(t *testing.T, input RenderInput) { assertEqual(t, input.Logger, logger) }},
		{"warning writer", WithWarningWriter(warningWriter), func(t *testing.T, input RenderInput) { assertEqual(t, input.WarningWriter, io.Writer(warningWriter)) }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			input := RenderInput{AssetInput: &AssetInput{CSS: TextSource{Text: "sentinel"}}}
			if err := test.option(&input); err != nil {
				t.Fatalf("option returned error: %v", err)
			}
			test.verify(t, input)
		})
	}

	fonts[0].Family = "mutated"
	if input, err := buildRenderInput("out.pdf", WithFontRegistrations(FontRegistration{Family: "stable"})); err != nil || input.FontRegistrations[0].Family != "stable" {
		t.Fatalf("font registrations were not defensively copied: input=%#v err=%v", input.FontRegistrations, err)
	}
	assetInput.HTML.Text = "mutated"
	if input, err := buildRenderInput("out.pdf", WithAssetInput(AssetInput{HTML: TextSource{Text: "stable"}})); err != nil || input.AssetInput.HTML.Text != "stable" {
		t.Fatalf("asset input was not copied: input=%#v err=%v", input.AssetInput, err)
	}
}

func TestBuildRenderInputDefaultsAndFailures(t *testing.T) {
	input, err := buildRenderInput("out.pdf", nil)
	if err != nil {
		t.Fatalf("buildRenderInput returned error: %v", err)
	}
	if input.OutputPath != "out.pdf" || input.DefaultLocale != DefaultLocale || input.DefaultCurrencyCode != DefaultCurrencyCode {
		t.Fatalf("unexpected defaults: %#v", input)
	}
	if _, err := buildRenderInput("  "); err == nil {
		t.Fatal("expected blank output path to fail")
	}
	wantErr := errors.New("option failed")
	if _, err := buildRenderInput("out.pdf", func(*RenderInput) error { return wantErr }); !errors.Is(err, wantErr) {
		t.Fatalf("buildRenderInput error = %v, want %v", err, wantErr)
	}
}

func TestRenderContextRejectsInvalidOutputBeforeRendering(t *testing.T) {
	if err := Render(" "); err == nil {
		t.Fatal("Render accepted an empty output path")
	}
	if err := RenderContext(context.Background(), " "); err == nil {
		t.Fatal("RenderContext accepted an empty output path")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := RenderContext(ctx, "out.pdf", WithAssets(minimalAssets())); !errors.Is(err, context.Canceled) {
		t.Fatalf("RenderContext error = %v, want context cancellation", err)
	}
}

func assertEqual(t *testing.T, got, want any) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}
