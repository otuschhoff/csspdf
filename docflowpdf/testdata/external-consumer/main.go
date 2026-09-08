package main

import (
	"log"

	"github.com/otuschhoff/csspdf/docflowpdf"
)

func main() {
	margins := docflowpdf.PageMargins{Top: 20, Right: 18, Bottom: 20, Left: 18}
	_ = docflowpdf.WithDefaultMargins(margins)
	assets := docflowpdf.Assets{
		HTML: `{{define "document"}}<div>Hello {{.Source.Name}}</div>{{end}}`,
		CSS:  `@page { size: A4; margin: 20pt; }`,
		Flow: docflowpdf.Flow{MainFlow: []docflowpdf.Section{{
			Template:    "document",
			Transformer: "generic",
			Payload:     docflowpdf.PayloadConfig{IncludeSource: true},
		}}},
		SourceData: map[string]any{"Name": "Docflow", "locale": "en"},
	}

	err := docflowpdf.RenderToFile(docflowpdf.RenderInput{
		Assets:              assets,
		DefaultMargins:      margins,
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
	}, "output.pdf")
	if err != nil {
		log.Fatal(err)
	}
}
