package csspdf

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

var benchmarkPagePattern = regexp.MustCompile(`/Type\s*/Page(?:\s|/)`)

func BenchmarkRenderWorkloads(b *testing.B) {
	fixedNow := func() time.Time { return time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC) }
	workloads := []struct {
		name  string
		input RenderInput
	}{
		{name: "small-letter", input: benchmarkSmallLetter(fixedNow)},
		{name: "layered-multi-section", input: benchmarkLayeredDocument(fixedNow)},
		{name: "long-table", input: benchmarkLongTable(fixedNow, 200)},
		{name: "images", input: benchmarkImageDocument(b, fixedNow)},
		{name: "utf8", input: benchmarkUTF8Document(fixedNow)},
	}
	for _, workload := range workloads {
		b.Run(workload.name, func(b *testing.B) { benchmarkRender(b, workload.input) })
	}
	b.Run("concurrent-independent", func(b *testing.B) {
		input := benchmarkLayeredDocument(fixedNow)
		var outputBytes, pages int64
		b.ReportAllocs()
		b.ResetTimer()
		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				pdf, err := RenderToBytes(input)
				if err != nil {
					b.Fatal(err)
				}
				outputBytes = int64(len(pdf))
				pages = int64(len(benchmarkPagePattern.FindAll(pdf, -1)))
			}
		})
		b.ReportMetric(float64(outputBytes), "output-B")
		b.ReportMetric(float64(pages), "pages")
	})
}

func benchmarkRender(b *testing.B, input RenderInput) {
	b.Helper()
	var outputBytes, pages int
	b.ReportAllocs()
	b.ResetTimer()
	for index := 0; index < b.N; index++ {
		pdf, err := RenderToBytes(input)
		if err != nil {
			b.Fatal(err)
		}
		outputBytes = len(pdf)
		pages = len(benchmarkPagePattern.FindAll(pdf, -1))
	}
	b.ReportMetric(float64(outputBytes), "output-B")
	b.ReportMetric(float64(pages), "pages")
}

func benchmarkSmallLetter(now func() time.Time) RenderInput {
	assets := minimalAssets()
	assets.HTML = `{{define "doc"}}<div><h1>Project update</h1><p>Hello {{.Source.Name}}</p><p>Prepared on {{formatDate .Source.Date}}</p></div>{{end}}{{define "page-number"}}<div>{{.Page}}/{{.Total}}</div>{{end}}`
	assets.CSS = `@page { size: A4; margin: 42pt; } h1 { font-size: 18pt; } p { font-size: 10pt; margin-bottom: 8pt; }`
	assets.SourceData = map[string]any{"Name": "Docflow", "Date": "2026-01-02", "locale": "en"}
	return RenderInput{Assets: assets, DefaultLocale: "en", DefaultCurrencyCode: "EUR", Now: now, FuncMapFactoryEx: DefaultTemplateFuncMapWithContext}
}

func benchmarkLayeredDocument(now func() time.Time) RenderInput {
	shared := `{{define "section"}}<div><h2>{{.Source.Title}}</h2><p>{{.Source.Body}}</p></div>{{end}}{{define "page-number"}}<div>{{.Page}}/{{.Total}}</div>{{end}}`
	assets := Assets{
		HTMLLayers: []HTMLLayer{{Name: "shared", HTML: shared}},
		HTML:       `{{define "summary"}}{{template "section" .}}{{end}}{{define "details"}}{{template "section" .}}{{end}}`,
		CSSLayers:  []CSSLayer{{Name: "base", CSS: `@page { size: A4; margin: 36pt; } div { font-size: 9pt; }`}, {Name: "document", CSS: `h2 { font-size: 14pt; color: #223344; } p { margin-bottom: 6pt; }`}},
		Flow: Flow{MainFlow: []Section{
			{Template: "summary", Transformer: "generic", Payload: PayloadConfig{IncludeSource: true}},
			{Template: "details", Transformer: "generic", Payload: PayloadConfig{IncludeSource: true}},
			{Template: "details", Transformer: "generic", Payload: PayloadConfig{IncludeSource: true}},
			{Template: "details", Transformer: "generic", Payload: PayloadConfig{IncludeSource: true}},
		}, PageNumber: Section{Template: "page-number", Transformer: "generic", Payload: PayloadConfig{Runtime: map[string]string{"Page": "page.number", "Total": "page.total"}}}},
		SourceData: map[string]any{"Title": "Layered report", "Body": strings.Repeat("Measured multi-section content. ", 20), "locale": "en"},
	}
	return RenderInput{Assets: assets, DefaultLocale: "en", DefaultCurrencyCode: "EUR", Now: now, FuncMapFactoryEx: DefaultTemplateFuncMapWithContext}
}

func benchmarkLongTable(now func() time.Time, rows int) RenderInput {
	var table strings.Builder
	table.WriteString(`{{define "doc"}}<table padding="2" row-height-min="10"><thead><tr><th>Item</th><th>Description</th><th>Amount</th></tr></thead><tbody>`)
	for row := 0; row < rows; row++ {
		fmt.Fprintf(&table, `<tr><td>%d</td><td>Measured table row %d</td><td>%.2f</td></tr>`, row+1, row+1, float64(row)/3)
	}
	table.WriteString(`</tbody></table>{{end}}{{define "page-number"}}<div>{{.Page}}/{{.Total}}</div>{{end}}`)
	assets := minimalAssets()
	assets.HTML = table.String()
	assets.CSS = `@page { size: A4; margin: 24pt; } table { width: 100%; table-layout: auto; } th { font-weight: bold; } td, th { font-size: 7pt; padding: 2pt; border: 0.5pt; }`
	assets.SourceData = map[string]any{"locale": "en"}
	return RenderInput{Assets: assets, DefaultLocale: "en", DefaultCurrencyCode: "EUR", Now: now}
}

func benchmarkImageDocument(b *testing.B, now func() time.Time) RenderInput {
	b.Helper()
	root := b.TempDir()
	var encoded bytes.Buffer
	picture := image.NewRGBA(image.Rect(0, 0, 64, 64))
	for y := 0; y < 64; y++ {
		for x := 0; x < 64; x++ {
			picture.Set(x, y, color.RGBA{R: uint8(x * 4), G: uint8(y * 4), B: 96, A: 255})
		}
	}
	if err := png.Encode(&encoded, picture); err != nil {
		b.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "benchmark.png"), encoded.Bytes(), 0o600); err != nil {
		b.Fatal(err)
	}
	assets := minimalAssets()
	assets.HTML = `{{define "doc"}}<div><img src="benchmark.png" width="64" height="64"><img src="benchmark.png" width="64" height="64"><img src="benchmark.png" width="64" height="64"><img src="benchmark.png" width="64" height="64"><p>Repeated image workload</p></div>{{end}}{{define "page-number"}}<div>{{.Page}}</div>{{end}}`
	assets.SourceData = map[string]any{"locale": "en"}
	return RenderInput{Assets: assets, DefaultLocale: "en", Now: now, ResourceResolver: ConfinedFileResolver{Root: root}}
}

func benchmarkUTF8Document(now func() time.Time) RenderInput {
	assets := minimalAssets()
	assets.HTML = `{{define "doc"}}<div><h1>Grüße aus Köln</h1><p>Crème brûlée, déjà vu, € 123,45.</p><p>{{.Source.Text}}</p></div>{{end}}{{define "page-number"}}<div>Seite {{.Page}}</div>{{end}}`
	assets.CSS = `@page { size: A4; margin: 36pt; } h1 { font-size: 16pt; } p { font-size: 10pt; }`
	assets.SourceData = map[string]any{"Text": strings.Repeat("Falsches Üben von Xylophonmusik quält jeden größeren Zwerg. ", 30), "locale": "de"}
	return RenderInput{Assets: assets, DefaultLocale: "de", DefaultCurrencyCode: "EUR", Now: now}
}
