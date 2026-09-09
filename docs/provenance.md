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

## Example Data and Images

`examples/layered` contains text-only synthetic demonstration content authored
for this repository. The example directory is not a stable API.

When adding an asset or fixture, record its origin, immutable source or author,
license, required attribution, redistribution approval, and whether it contains
real personal or customer data. Assets missing any required evidence must stay
outside version control and release archives.

## Utility Support Status

| Component | Status | Validation |
| --- | --- | --- |
| `csspdf` | Supported public Go package | Full tests, external-consumer build/render, race and coverage gates |
| `cmd/gen-example` | Supported repository tool | Layered smoke test and rollout script |
| `cmd/dom-parse` | Supported repository tool | Writer-injected usage, error, and DOM output tests |
| `cmd/pdfdump` | Supported bounded diagnostic tool, not a conformance validator | CLI tests and shared `internal/pdfdump` fuzzing |
| generated files under `bin/` and `output/` | Unsupported artifacts | Excluded from source releases |
