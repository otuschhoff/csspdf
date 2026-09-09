# csspdf

A Go library for generating PDF files from:
- HTML template fragments
- CSS
- flow JSON (section orchestration + payload transforms)
- source data (JSON or Go object)
- optional Go template functions

The reusable public package is:
- github.com/otuschhoff/csspdf

## Requirements and Dependency Policy

- Go 1.26.6 or newer
- No sibling repositories or local Go workspace are required
- Dependencies are pinned in `go.mod` and `go.sum`

The PDF backend is the pinned `github.com/otuschhoff/gofpdf` fork dependency.
The required APIs, UTF-8 fixes, and their regression tests are maintained in
that repository; csspdf does not require a sibling checkout or local replace.

Supported release lines, compatibility rules, deprecations, provenance, and
release checks are documented in [API compatibility](docs/architecture/api-compatibility.md),
[asset provenance](docs/maintenance/provenance.md), and the
[release procedure](docs/maintenance/release.md). The complete documentation
map is in [docs/README.md](docs/README.md). The project is distributed under the
[MIT License](LICENSE).

## Status

This repository now separates:
- Generic rendering engine: csspdf
- Rendering internals: internal/pdfrender, internal/pdfdom, internal/templating, internal/flowrender
- Layered CSS example: examples/layered

## Quick Start

```go
package main

import (
	"log"

	"github.com/otuschhoff/csspdf"
)

func main() {
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
		DefaultLocale:       "en",
		DefaultCurrencyCode: "EUR",
	}, "output.pdf")
	if err != nil {
		log.Fatal(err)
	}
}
```

## Public API Overview

Primary package:
- csspdf

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

## Confined Rendering

For service workloads, confine file-backed resources and set explicit budgets:

```go
ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
defer cancel()

if err := os.MkdirAll(".build/output", 0o755); err != nil {
	log.Fatal(err)
}
err := csspdf.RenderContext(ctx, ".build/output/document.pdf",
	csspdf.WithConfinedResourceRoot("/srv/csspdf/jobs/job-123"),
	csspdf.WithRenderLimits(csspdf.RenderLimits{
		SourceBytes:         4 << 20,
		TemplateOutputBytes: 8 << 20,
		ImageBytes:          8 << 20,
		ImagePixels:         12_000_000,
		OutputBytes:         32 << 20,
		Nodes:               25_000,
		Depth:               128,
		Rows:                10_000,
		Pages:               250,
	}),
)
```

Zero limit fields use safe library defaults. Budget violations return
`*csspdf.BudgetError`; file output remains atomic. Templates and custom Go
functions remain trusted code, and hard resource ceilings require process or
container isolation. See [the security contract](docs/architecture/security.md).

## Layered CSS Usage

You can compose CSS in ordered layers with `AssetInput.CSSLayers`.
Later layers override earlier mapped declarations.

csspdf intentionally implements a documented CSS subset rather than a browser
cascade. Use `csspdf.SupportedCSSProperties()` for the executable property
matrix and `csspdf.AnalyzeCSSSupport(cssText)` to detect ignored properties
(`CSS001`) before rendering. Rules of equal or different selector shapes use
source order; selector specificity and `!important` are not interpreted.

```go
assetInput := csspdf.AssetInput{
	HTML:       csspdf.TextSource{FilePath: "examples/layered/doc.html"},
	Flow:       csspdf.JSONSource{FilePath: "examples/layered/flow.json"},
	SourceData: csspdf.JSONSource{FilePath: "examples/layered/data.json"},
	CSSLayers: []csspdf.CSSLayerInput{
		{Name: "corporate-base", Source: csspdf.TextSource{FilePath: "examples/layered/styles/corporate/base.css"}},
		{Name: "document", Source: csspdf.TextSource{FilePath: "examples/layered/styles/document/doc.css"}},
		{Name: "customer-override", Source: csspdf.TextSource{FilePath: "examples/layered/styles/overrides/customer.css"}, Optional: true},
	},
}

if err := os.MkdirAll(".build/output", 0o755); err != nil {
	log.Fatal(err)
}
err := csspdf.Render(".build/output/layered.pdf",
	csspdf.WithAssetInput(assetInput),
	csspdf.WithDefaultLocale("en"),
	csspdf.WithDefaultCurrencyCode("EUR"),
	csspdf.WithFuncMapFactoryEx(csspdf.DefaultTemplateFuncMapWithContext),
)
if err != nil {
	log.Fatal(err)
}
```

Run the included end-to-end layered profile example:

```bash
go run ./cmd/csspdf gen-example layered -o .build/output/layered.pdf
```

Render the 20-case office and business document gallery:

```bash
go run ./cmd/csspdf gen-example office-suite -o .build/output/office-suite
```

The gallery includes memos, letters, receipts, quotes, orders, invoices,
reports, contracts, a landscape catalog, a high-volume stress document, and
three asserted failure cases. See [the office-suite guide](examples/office-suite/README.md).

Notes:
- If both `CSSLayers` and legacy `Assets.CSS` are set, legacy CSS is applied as an implicit final layer.
- csspdf uses mapped-property style application, not full browser cascade semantics.
- HTML and CSS support a deliberate document-rendering subset. Do not assume
	browser-equivalent selector, cascade, layout, scripting, or network behavior.

Migration helper example:

```go
legacyInput := csspdf.AssetInput{
	HTML: csspdf.TextSource{FilePath: "templates/doc.html"},
	CSS:  csspdf.TextSource{FilePath: "templates/doc.css"},
	Flow: csspdf.JSONSource{FilePath: "templates/flow.json"},
}

layeredInput := csspdf.MigrateAssetInputLegacyCSSToSingleLayer(legacyInput, "legacy")
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
assetInput := csspdf.AssetInput{
	HTML: csspdf.TextSource{FilePath: "templates/invoice.content.html"},
	HTMLLayers: []csspdf.HTMLLayerInput{
		{
			Name: "shared-shell",
			Source: csspdf.TextSource{FilePath: "templates/default.shell.html"},
		},
	},
	CSS:        csspdf.TextSource{FilePath: "templates/doc.css"},
	Flow:       csspdf.JSONSource{FilePath: "templates/flow.json"},
	SourceData: csspdf.JSONSource{FilePath: "templates/data.json"},
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
- WarningWriter (deprecated; removed in v0.3.0, see docs/architecture/api-compatibility.md)

The warning sink intentionally remains a minimal `Warnf` compatibility
interface. Typed render failures use `DiagnosticError`; applications that need
structured logs should inspect those errors with `errors.As` and adapt warning
messages at their boundary rather than parsing error strings.

All configured resource and processing limit failures match
`csspdf.ErrLimitExceeded` through wrapping. Use
`errors.Is(err, csspdf.ErrLimitExceeded)` for the general category and
`errors.As` with `BudgetError` or `LimitError` when concrete limit details are
required. File-source errors preserve their underlying causes, including
`fs.ErrNotExist` and `fs.ErrPermission`.

Rendering is strict by default. Missing templates, missing map values, missing
images, invalid numeric span attributes, and element-rendering failures return
errors instead of producing an incomplete PDF. Applications that temporarily
require the previous behavior can set `RenderInput.AllowPartialRender` or use
`WithLegacyPartialRendering(true)`; recoverable failures are then sent to the
warning sink while rendering continues. This legacy path is deprecated and
scheduled for removal in v0.3.0.

`RenderToFile` renders before touching the destination and replaces it through
a temporary file in the destination directory. Existing file permissions are
preserved; new files are owner-only (`0600`). `RenderToWriter` cannot roll back
bytes already accepted by an arbitrary writer. See
`docs/guides/migration-v0.2.md` for the complete output contract.

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

## Layout Guarantees

Consecutive flow sections share a persistent normal-flow cursor and therefore
do not overlap. Absolute elements and page-number overlays are page-local and
do not alter that cursor.

Page dimensions and margins must define a finite, positive content box.
Normal-flow tables paginate between rows and repeat leading `thead` rows on
each page. Colspan occupancy is shared by width inference, row measurement,
and rendering. A table row or image that cannot fit on an empty content box
returns an overflow error rather than rendering beyond the page. Long text
continues across pages without rewrapping already rendered lines.

See [the layout contract](docs/architecture/layout.md) for detailed overflow
and compatibility behavior.

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

`RenderInput.Now` can inject a custom clock. When it is non-nil, the returned
time also fixes the PDF creation and modification metadata. The renderer emits
resource catalogs in stable order, so repeated renders are byte-identical when
templates, data, resources, options, and custom functions are themselves
deterministic.

When `FuncMapFactoryEx` is used, the function context receives `Now`. With no
clock configured, metadata uses the current time and only semantic output is
expected to be stable; byte identity is not part of the default contract.

## Example Profile

Layered CSS concept assets live in:
- examples/layered

This keeps csspdf generic while demonstrating layered style composition.

## Testing

Run the complete local quality gate:

```bash
./scripts/check-quality.sh
```

The gate checks module tidiness, dependency checksums, formatting, builds,
static analysis, root-module tests, package coverage floors, and reachable
vulnerabilities.
The vulnerability tool is pinned; set `RUN_VULN_CHECK=false` only for a fast
local iteration after an unchanged successful scan. The initial scan and
clean-checkout evidence are recorded in `docs/history/phase0-baseline.md`.

Release-readiness evidence and troubleshooting guidance are recorded in
`docs/maintenance/release-readiness.md` and `docs/guides/troubleshooting.md`.

Run the changed-code maintainability ratchet with:

```bash
go install github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0
./scripts/check-maintainability.sh
```

Run the measured performance corpus with:

```bash
./scripts/benchmark-phase5.sh
```

Reference results, workload definitions, the regression budget, and the
determinism contract are recorded in `docs/maintenance/performance.md`.

## Tasks

Run tasks with `xc <task>`. The task definitions below are xc-compatible.

### build

Build the CLI binary into `.build/bin/`. Repository-local binaries, rendered
examples, and persistent profiles belong under the ignored `.build/` tree;
self-cleaning test and coverage files remain in the operating-system temp
directory.

```sh
mkdir -p .build/bin
go build -o .build/bin/csspdf ./cmd/csspdf
```

### render-sample

Build and render the layered sample document.

```sh
mkdir -p .build/bin
go build -o .build/bin/csspdf ./cmd/csspdf
./.build/bin/csspdf gen-example layered -o .build/output/layered.pdf
```

### test

Run the same test suites used by CI.

```sh
go test ./... -count=1
```

## Supported Components

- `csspdf` is the supported library API.
- `cmd/csspdf` is the supported repository tool, with `gen-example`,
  `dom-parse`, and `pdfdump` commands.
- The `csspdf pdfdump` command is supported for bounded inspection;
	it is not a PDF conformance validator.
- `examples/layered` contains reference assets, not stable Go APIs.
- Generated binaries and output files are unsupported artifacts and are ignored
	by version control.
