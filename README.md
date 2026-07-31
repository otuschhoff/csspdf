# docflowpdf Library (invoice-gen)

A Go library for generating PDF files from:
- HTML template fragments
- CSS
- flow JSON (section orchestration + payload transforms)
- source data (JSON or Go object)
- optional Go template functions

The reusable public package is:
- github.com/otuschhoff/invoice-gen/docflowpdf

## Status

This repository now separates:
- Generic rendering engine: docflowpdf
- Rendering internals: internal/pdfrender, internal/pdfdom, internal/template, internal/templateflow
- Invoice profile/example: examples/invoice

## Quick Start

```go
package main

import (
	"log"
	"os"

	"github.com/otuschhoff/invoice-gen/docflowpdf"
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
- Render(input): convenience method writing to input.OutputPath
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
go test ./docflowpdf ./examples/invoice ./cmd/invoice-gen ./internal/pdfrender
```

## Tasks

Run tasks with `xc <task>`. The task definitions below are xc-compatible.

### build

Build CLI binaries into `bin/`.

```sh
go build -o bin/invoice-gen ./cmd/invoice-gen
go build -o bin/dom-parse ./cmd/dom-parse
```

### render-sample

Build and render the sample totals document.

```sh
go build -o bin/invoice-gen ./cmd/invoice-gen
./bin/invoice-gen totals -o output/totals.pdf
```

### test

Run focused tests used during the library refactor.

```sh
go test ./docflowpdf ./examples/invoice ./cmd/invoice-gen ./internal/pdfrender ./internal/i18n
```

## Notes

- Some legacy repository commands/packages are still present for historical tooling.
- If you only want to consume the library, use the docflowpdf package and treat invoice profile code as optional profile/example material.
