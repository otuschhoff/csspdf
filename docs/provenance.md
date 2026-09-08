# Asset, Fixture, and Utility Provenance

## Repository Source License

The csspdf repository does not currently contain a root license file. Public
source or binary redistribution is blocked until the repository owner selects
and adds a license. The release checklist treats this as an explicit owner
decision; no license is inferred from dependencies.

## Repository-Owned PDF Backend

`third_party/gofpdf` is based on the immutable revision and local patches listed
in `third_party/gofpdf/PATCHES.md`. Its upstream MIT and ISC license files are
retained in that nested module. Those licenses apply to that source, not to the
rest of csspdf.

## Test Fonts

The UTF-8 integration fixture comes from
`golang.org/x/image/font/gofont/goregular`, pinned by the root module. It is
generated into a test-owned temporary directory and is not copied into release
artifacts.

The previously bundled `examples/invoice/fonts/Futura-Medium.ttf` declared
"ALL RIGHTS RESERVED" and had no redistribution grant in this repository. It
was removed in Phase 6 and the example now uses a standard PDF font.

## Example Data and Images

`examples/invoice/data.json` is synthetic demonstration data. Names,
addresses, contact details, tax and banking identifiers, customer references,
and work descriptions use explicit example values and reserved domains. It
must not be replaced with production invoice or timesheet data.

Signature image derivatives and their XCF/SVG sources previously under
`examples/invoice/images` had no recorded redistribution provenance and were
removed in Phase 6. The invoice example no longer renders a signature image.

`examples/layered` contains text-only synthetic demonstration content authored
for this repository. Neither example directory is a stable API.

When adding an asset or fixture, record its origin, immutable source or author,
license, required attribution, redistribution approval, and whether it contains
real personal or customer data. Assets missing any required evidence must stay
outside version control and release archives.

## Utility Support Status

| Component | Status | Validation |
| --- | --- | --- |
| `docflowpdf` | Supported public Go package | Full tests, external-consumer build/render, race and coverage gates |
| `cmd/gen-example` | Supported repository tool | Invoice/layered smoke tests and rollout script |
| `cmd/dom-parse` | Supported repository tool | Writer-injected usage, error, and DOM output tests |
| root `pdfdump.go` | Supported bounded diagnostic tool, not a conformance validator | CLI tests and shared `internal/pdfdump` fuzzing |
| `genXml.js`, `mkDoc.js`, `mkQuote.js` | Unsupported legacy utilities | No dependency manifest or CI; excluded from release support claims |
| generated files under `bin/` and `output/` | Unsupported artifacts | Excluded from source releases |
