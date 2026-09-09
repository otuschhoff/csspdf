package main

import (
	"log"

	"github.com/otuschhoff/csspdf"
)

func main() {
	margins := csspdf.PageMargins{Top: 20, Right: 18, Bottom: 20, Left: 18}
	_ = csspdf.WithDefaultMargins(margins)
	assets := csspdf.Assets{
		HTML: `{{define "document"}}<div>Hello {{.Source.Name}}</div>{{end}}`,
		CSS:  `@page { size: A4; margin: 20pt; }`,
		Flow: csspdf.Flow{MainFlow: []csspdf.Section{{
			Template:    "document",
			Transformer: "generic",
			Payload:     csspdf.PayloadConfig{IncludeSource: true},
		}}},
		SourceData: map[string]any{"Name": "Docflow", "locale": "en"},
	}

	err := csspdf.RenderToFile(csspdf.RenderInput{
		Assets:              assets,
		DefaultMargins:      margins,
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
	}, "output.pdf")
	if err != nil {
		log.Fatal(err)
	}
}
