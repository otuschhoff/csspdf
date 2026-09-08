package main

import "github.com/otuschhoff/csspdf/docflowpdf"

func main() {
	margins := docflowpdf.PageMargins{Top: 20, Right: 18, Bottom: 20, Left: 18}
	input := docflowpdf.RenderInput{DefaultMargins: margins}
	_ = input
	_ = docflowpdf.WithDefaultMargins(margins)
}
