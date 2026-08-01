# docflowpdf Library (go-dom2pdf)

A Go library for generating PDF files from:
- HTML template fragments
- CSS
- flow JSON (section orchestration + payload transforms)
- source data (JSON or Go object)
- optional Go template functions

The reusable public package is:
- github.com/otuschhoff/go-dom2pdf/docflowpdf

## Status

This repository now separates:
- Generic rendering engine: docflowpdf
- Rendering internals: internal/pdfrender, internal/pdfdom, internal/templating, internal/flowrender
- Invoice profile/example: examples/invoice

## Quick Start

```go
package main

import (
	"log"
	"os"

	"github.com/otuschhoff/go-dom2pdf/docflowpdf"
)

func main() {
	assets, err := (docflowpdf.AssetInput{
		HTML: docflowpdf.TextSource{FilePath: "templates/doc.html.tmpl"},
		CSS:  docflowpdf.TextSource{FilePath: "templates/doc.css"},
		Flow: docflowpdf.JSONSource{FilePath: "templates/doc.flow.json"},
		SourceData: docflowpdf.JSONSource{FilePath: "data/doc.data.json"},
	}).ResolveAssets()
	if err != nil {
		log.Fatal(err)
	}

	f, err := os.Create("output.pdf")
	if err != nil {
		log.Fatal(err)
	}
	defer f.Close()

	err = docflowpdf.RenderToWriter(docflowpdf.RenderInput{
		Assets:              assets,
		PageCount:           1,
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
	}, f)
	if err != nil {
		log.Fatal(err)
	}
}
```

## Public API Overview

Primary package:
- docflowpdf

Core methods:
- Render(outputPath, options...): convenience API that builds RenderInput internally
- RenderWithInput(input): full-struct API for advanced control
- RenderToFile(input, outputPath)
- RenderToWriter(input, writer)
- RenderToBytes(input)

Input models:
- RenderInput
- Assets
- AssetInput
- TextSource
- JSONSource

I18n source input:
- RenderInput.I18nSource also uses JSONSource, so translations can come from object, raw/text JSON, file path, or io/fs.

Template functions:
- Legacy hook: FuncMapFactory(defaultLocale, payloadLocale)
- Context-aware hook: FuncMapFactoryEx(FuncContext) with deterministic Now function
- Reusable default helpers: DefaultTemplateFuncMap(defaultLocale, payloadLocale)
- Context-aware default helpers: DefaultTemplateFuncMapWithContext(FuncContext)

Date formatting helpers available in templates:
- `formatLocalizedDate value style [localeOverride]`
- `formatLocalizedDateOrNow value style [localeOverride]`

Supported styles:
- `written-month` (month name only)
- `local` (locale-default full date)
- `short` (numeric short date)
- `datetime` (date + time)
- `layout:<go time layout>` (custom layout)

Examples:

```gotemplate
{{formatLocalizedDate .Source.Invoice.Date "local"}}
{{formatLocalizedDate .Source.Invoice.Date "local" "de"}}
{{formatLocalizedDate .Source.Invoice.Date "written-month"}}
{{formatLocalizedDate .Source.Invoice.Date "short"}}
{{formatLocalizedDate .Source.Invoice.Date "datetime"}}
{{formatLocalizedDate .Source.Invoice.Date "layout:2006-01-02"}}
{{formatLocalizedDateOrNow .Source.DocDate "local"}}
```

Typical output examples:
- `de` + `local` => `31. Juli 2026`
- `en` + `local` => `July 31 2026`
- `de` + `written-month` => `Juli`
- `en` + `written-month` => `July`

Optional i18n macro execution:
- Enable with `WithI18nTemplateMacros(true)`.
- When enabled, i18n string values are treated as Go templates and can use the
	same helper functions as document templates.
- i18n templates can reference values from `data.json` via `.Source`, for
	example:

```json
{
	"invoiceIntro": {
		"en": "Services in {{formatLocalizedDateOrNow .Source.Invoice.Date \"written-month\"}}"
	}
}
```

## Asset Input Modes

TextSource supports:
- Raw []byte
- Text string
- FilePath
- io/fs (FS + FSPath)

JSONSource supports:
- Object (Go struct/map/slice)
- Raw []byte
- Text string
- FilePath
- io/fs (FS + FSPath)

This avoids mandatory caller-side JSON serde when using the library in Go apps.

## Flow JSON Validation

Flow validation now checks:
- Required sections/templates
- Supported transformers (currently generic)
- Payload path syntax
- Runtime expression syntax

Supported runtime expressions:
- flow.tableWidth
- flow.remainingWidth:<comma-separated offsets>
- page.number
- page.total

## Warnings and Diagnostics

RenderInput supports warning sinks via:
- Logger (preferred)
- WarningWriter (deprecated compatibility path)

Non-fatal render issues (for example, page-number template render failures) are emitted as warnings while rendering continues.

## Deterministic Rendering Support

RenderInput.Now can inject a custom clock.

When FuncMapFactoryEx is used, the function context receives Now, enabling deterministic template function behavior in tests and reproducible builds.

## Invoice Profile

Invoice assets and helper functions live in:
- examples/invoice

This keeps docflowpdf generic while preserving an invoice profile implementation.

## Testing

Focused package tests:

```bash
go test ./docflowpdf ./examples/invoice ./cmd/gen-example ./internal/pdfrender
```

## Tasks

Run tasks with `xc <task>`. The task definitions below are xc-compatible.

### build

Build CLI binaries into `bin/`.

```sh
go build -o bin/gen-example ./cmd/gen-example
go build -o bin/dom-parse ./cmd/dom-parse
```

### render-sample

Build and render the sample invoice document.

```sh
go build -o bin/gen-example ./cmd/gen-example
./bin/gen-example invoice -o output/invoice.pdf
```

### test

Run focused tests used during the library refactor.

```sh
go test ./docflowpdf ./examples/invoice ./cmd/gen-example ./internal/pdfrender ./internal/i18n
```

## Notes

- Some legacy repository commands/packages are still present for historical tooling.
- If you only want to consume the library, use the docflowpdf package and treat invoice profile code as optional profile/example material.
