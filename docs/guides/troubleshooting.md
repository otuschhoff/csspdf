# Troubleshooting and Support Boundaries

## Start With the Shared Gates

Run `scripts/check-quality.sh` from the repository root. It checks module
manifests, formatting, documentation links, builds, vet, root-module tests,
package coverage floors, and the pinned reachable-vulnerability scan. Use
`RUN_VULN_CHECK=false` only after an unchanged successful scan.

For rendering failures, preserve errors with `%w` and inspect them with
`errors.As`. `*csspdf.RenderError` identifies the stage and available
section, page, layer, and resource context. `*csspdf.BudgetError` identifies
which configured limit was exceeded. Library errors do not include ANSI color.

## Required Content and Partial Rendering

Missing templates, values, images, reusable templates, and rendering failures
are errors by default. `WithLegacyPartialRendering(true)` is a compatibility
mode that can produce incomplete PDFs; it is not a general recovery mechanism.
Use a logger to retain warnings and migrate the underlying asset or template.

## Files and Resources

An unexpected `resource path escapes confined root` error means the resolved
path is absolute, traverses a parent, follows a symlink outside the root, or is
not a regular file. Service integrations should use
`WithConfinedResourceRoot`. `WithTrustedFileAccess` permits host filesystem
access and is intended only for trusted local authoring.

File output is atomic for ordinary process failures. If replacement fails,
check destination-directory permissions and host `os.Rename` behavior. Writer
output cannot retract bytes already accepted by the caller.

## HTML and CSS

csspdf is not a browser engine. It supports document-oriented HTML elements and
a mapped CSS subset. Use `csspdf.SupportedCSSProperties()` and
`csspdf.AnalyzeCSSSupport` to inspect declarations. Unsupported properties
produce `CSS001` diagnostics when warning analysis is enabled. Selector
specificity, `!important`, scripts, network fetching, and browser layout are
not supported.

Root flow elements are `div`, `footer`, `p`, `table`, `img`, and supported typed
value/template elements. Tables require valid column spans and are constrained
by row/page budgets. See `docs/concepts/layered-css-organization.md` and
`docs/architecture/layout.md` for precedence and pagination contracts.

## Fonts and Text

Standard PDF fonts use CP-1252-compatible text. Register a TTF, OTF, or TTC font
for broader Unicode coverage. A font must be supplied through an authorized
resource source and must be licensed for the intended distribution. The
integration suite uses the redistributable Go font fixture from
`golang.org/x/image/font/gofont/goregular`.

## Reproducibility and Performance

Set `RenderInput.Now` to a fixed function for byte-reproducible output. Without
it, PDF metadata uses the current time and only semantic stability is expected.
Custom functions, resource content, and input data must also be deterministic.

Use `scripts/benchmark-phase5.sh` on an idle reference runner for performance
investigation. Timing is not a shared-runner CI threshold. Page count, output
size, allocations, semantic tests, and the package coverage ratchet provide the
stable automated checks.

## Security Reports

Do not include private templates, source data, credentials, or customer PDFs in
public reports. Provide the failing stage/code, sanitized input shape, Go and OS
versions, configured limits, and a minimal reproduction. This repository does
not claim hostile-template sandboxing; isolate untrusted workloads at the
process or container boundary.
