# docflowpdf Library (csspdf)

A Go library for generating PDF files from:
- HTML template fragments
- CSS
- flow JSON (section orchestration + payload transforms)
- source data (JSON or Go object)
- optional Go template functions

The reusable public package is:
- github.com/otuschhoff/csspdf/docflowpdf

## Requirements and Dependency Policy

- Go 1.25.13 or newer
- No sibling repositories or local Go workspace are required
- Dependencies are pinned in `go.mod` and `go.sum`

The PDF backend is maintained as repository-owned source in
`third_party/gofpdf` because csspdf depends on fork APIs and UTF-8 fixes that
are not all available from the fork's published branch. Its exact source
revision, local patch set, and retained test scope are documented in
`third_party/gofpdf/PATCHES.md`.

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

	"github.com/otuschhoff/csspdf/docflowpdf"
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

## Layered CSS Usage

You can compose CSS in ordered layers with `AssetInput.CSSLayers`.
Later layers override earlier mapped declarations.

```go
assetInput := docflowpdf.AssetInput{
	HTML:       docflowpdf.TextSource{FilePath: "examples/layered/doc.html"},
	Flow:       docflowpdf.JSONSource{FilePath: "examples/layered/flow.json"},
	SourceData: docflowpdf.JSONSource{FilePath: "examples/layered/data.json"},
	CSSLayers: []docflowpdf.CSSLayerInput{
		{Name: "corporate-base", Source: docflowpdf.TextSource{FilePath: "examples/layered/styles/corporate/base.css"}},
		{Name: "document", Source: docflowpdf.TextSource{FilePath: "examples/layered/styles/document/doc.css"}},
		{Name: "customer-override", Source: docflowpdf.TextSource{FilePath: "examples/layered/styles/overrides/customer.css"}, Optional: true},
	},
}

err := docflowpdf.Render("output/layered.pdf",
	docflowpdf.WithAssetInput(assetInput),
	docflowpdf.WithDefaultLocale("en"),
	docflowpdf.WithDefaultCurrencyCode("EUR"),
	docflowpdf.WithFuncMapFactoryEx(docflowpdf.DefaultTemplateFuncMapWithContext),
)
if err != nil {
	log.Fatal(err)
}
```

Run the included end-to-end layered profile example:

```bash
go run ./cmd/gen-example layered -o output/layered.pdf
```

Notes:
- If both `CSSLayers` and legacy `Assets.CSS` are set, legacy CSS is applied as an implicit final layer.
- csspdf uses mapped-property style application, not full browser cascade semantics.
- HTML and CSS support a deliberate document-rendering subset. Do not assume
	browser-equivalent selector, cascade, layout, scripting, or network behavior.

Migration helper example:

```go
legacyInput := docflowpdf.AssetInput{
	HTML: docflowpdf.TextSource{FilePath: "templates/doc.html"},
	CSS:  docflowpdf.TextSource{FilePath: "templates/doc.css"},
	Flow: docflowpdf.JSONSource{FilePath: "templates/flow.json"},
}

layeredInput := docflowpdf.MigrateAssetInputLegacyCSSToSingleLayer(legacyInput, "legacy")
```

Rollout checks:

```bash
./scripts/check-layered-rollout.sh
```

The rollout script:
- runs focused tests
- renders legacy and layered examples
- reports PDF sizes
- prints a layered CSS duplication metric

## Layered HTML Shell Usage

You can compose HTML templates in ordered layers with `AssetInput.HTMLLayers`.
Layers are parsed first, then legacy `AssetInput.HTML` is parsed last as an
implicit final layer, so document templates can override shared block defaults.

```go
assetInput := docflowpdf.AssetInput{
	HTML: docflowpdf.TextSource{FilePath: "templates/invoice.content.html"},
	HTMLLayers: []docflowpdf.HTMLLayerInput{
		{
			Name: "shared-shell",
			Source: docflowpdf.TextSource{FilePath: "templates/default.shell.html"},
		},
	},
	CSS:        docflowpdf.TextSource{FilePath: "templates/doc.css"},
	Flow:       docflowpdf.JSONSource{FilePath: "templates/flow.json"},
	SourceData: docflowpdf.JSONSource{FilePath: "templates/data.json"},
}
```

Shared shell example (`default.shell.html`):

```gotemplate
{{define "doc"}}
<div id="page">
	{{block "default-letterhead" .}}<div>ACME Corp</div>{{end}}
	{{block "document-content" .}}<div>Default body</div>{{end}}
	{{block "default-footer" .}}<div>Default footer</div>{{end}}
</div>
{{end}}

{{define "page-number"}}<div>{{.page.pageNumber}} / {{.page.pageNumberTotal}}</div>{{end}}
```

Document content example (`invoice.content.html`):

```gotemplate
{{define "document-content"}}<div>Invoice {{.Source.Invoice.ID}}</div>{{end}}
{{define "default-footer"}}<div>Invoice-specific footer</div>{{end}}
```

Notes:
- `HTMLLayers` are optional; existing single-file HTML templates keep working unchanged.
- If a wrapper references missing nested templates (for example `document-content`), render fails with a template error.
- This composition model works with `CSSLayers`, so shared markup and shared styles can both be centralized.

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

Rendering is strict by default. Missing templates, missing map values, missing
images, and element-rendering failures return errors instead of producing an
incomplete PDF. Applications that temporarily require the previous behavior
can set `RenderInput.AllowPartialRender` or use
`WithLegacyPartialRendering(true)`; recoverable failures are then sent to the
warning sink while rendering continues.

`RenderToFile` renders before touching the destination and replaces it through
a temporary file in the destination directory. Existing file permissions are
preserved; new files are owner-only (`0600`). `RenderToWriter` cannot roll back
bytes already accepted by an arbitrary writer. See
`docs/phase1-migration.md` for the complete output contract.

## Input Normalization

JSON numbers decoded into dynamic values use `json.Number`, preserving their
source representation and integers above $2^{53}$. Custom template functions
should use `json.Number.String`, `Int64`, or `Float64` according to whether the
value is an identifier, integer, or quantity. Generic numeric helpers accept
both `json.Number` and native Go numeric values.

Flow JSON rejects unknown fields. Runtime and static payload targets cannot
overlap, use prefix-conflicting paths, or overwrite the reserved roots
`Source`, `Payload`, `page`, `locale`, and `i18n`.

Without an explicit i18n source, formatting uses self-contained separators:
German uses decimal comma and thousands point; other locales use decimal point
and thousands comma. `AssetBaseDir/i18n.json` remains an explicit profile
default when an asset base directory is configured.

Currency formatting rounds the complete value once using the precision of the
currency: JPY uses zero decimals, BHD and KWD use three, and other currencies
use two. Rounded negative zero is emitted without a minus sign. Float inputs
retain IEEE 754 semantics; applications requiring exact financial arithmetic
should calculate exact minor units before passing display values to csspdf.

## Security and Trust Model

The current release is intended for trusted local authoring. Treat HTML/CSS
templates, flow definitions, asset paths, font registrations, and custom Go
template functions as trusted configuration. They are not sandboxed.

Do not expose the renderer directly to tenant- or user-supplied templates or
asset paths. File sources and image/font lookup can access the host filesystem,
including configured absolute paths and compatibility search locations. The
renderer does not currently enforce input-size, page-count, image-size, or
execution-time limits. Use process isolation and application-level limits when
rendering data from less-trusted sources. See `SECURITY.md` for the complete
current boundary.

## Deterministic Rendering Support

RenderInput.Now can inject a custom clock.

When FuncMapFactoryEx is used, the function context receives Now, enabling deterministic template function behavior in tests and reproducible builds.

## Invoice Profile

Invoice assets and helper functions live in:
- examples/invoice

Layered CSS concept/example assets live in:
- examples/layered

This keeps docflowpdf generic while preserving an invoice profile implementation.

## Testing

Run the complete local quality gate:

```bash
./scripts/check-quality.sh
```

The gate checks module tidiness, dependency checksums, formatting, builds,
static analysis, all root and backend tests, and reachable vulnerabilities.
The vulnerability tool is pinned; set `RUN_VULN_CHECK=false` only for a fast
local iteration after an unchanged successful scan. The initial scan and
clean-checkout evidence are recorded in `docs/phase0-baseline.md`.

Run the changed-code maintainability ratchet with:

```bash
go install github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0
./scripts/check-maintainability.sh
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

Run the same test suites used by CI.

```sh
go test ./... -count=1
(cd third_party/gofpdf && go test ./... -count=1)
```

## Supported Components

- `docflowpdf` is the supported library API.
- `cmd/gen-example` and `cmd/dom-parse` are supported repository tools.
- `examples/invoice` and `examples/layered` are reference assets, not stable Go APIs.
- Root JavaScript files (`genXml.js`, `mkDoc.js`, and `mkQuote.js`) are legacy,
	unsupported utilities. They have no maintained package manifest or CI gate.
- Generated binaries and output files are unsupported artifacts and are ignored
	by version control.
