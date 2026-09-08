# Code Quality Review and Remediation Plan

Review date: 2026-09-07. Baseline commit: `6fd92d3`.

## Recommendation

Stabilize the build and document-correctness contract before expanding features or reorganizing the renderer. The project has useful package boundaries, flexible input APIs, and targeted regression tests, but it is not yet a reliable general-purpose document library: valid-looking PDFs can contain missing, overlapping, off-page, or incorrectly formatted content without an error.

The most important work is not cosmetic. Fix reproducibility, error propagation, output preservation, monetary formatting, and flow layout first. Then establish explicit trust boundaries, improve diagnostics, and refactor behind executable behavior tests. Do not undertake a renderer rewrite or implement a full browser CSS engine as part of this program.

This report covers code quality, structure, maintainability, security, scalability, debuggability, correctness, and engineering practices. It is an engineering assessment, not a security certification, legal assessment, PDF/A conformance audit, or exhaustive review of every branch.

## Findings

Severity meanings:

- **P1:** Release-blocking build, data-integrity, or document-correctness issue for the affected supported workflow.
- **P1 conditional:** Blocker if the application accepts assets or workloads from untrusted parties.
- **P2:** Important reliability, maintainability, or operational gap to address before calling the library production-hardened.

Evidence labels distinguish **reproduced** behavior from **inspection** findings. Proposed tests below are acceptance criteria, not claims that those tests currently exist or pass.

### R01. Builds and Tests Depend on Unpinned Sibling Checkouts

**P1 | Reproduced | Reproducibility, maintainability**

Evidence: [go.mod](../go.mod#L1), [font integration test](../docflowpdf/font_emission_integration_test.go#L80), and [CI toolchain](../.github/workflows/maintainability.yml#L21).

`go test ./... -count=1 -cover` stops with `updates to go.mod needed`. The committed module replaces both the PDF backend and PDF/A library with sibling directories. Resolving against the checkouts on this machine using a temporary module file raises the Go directive from 1.25.0 to 1.26.0 and changes numerous dependency versions. Tests then fail to compile because `ExtractFontsFromPDF` and `ExtractUnicodesForFont` are absent from the local PDF/A converter's public API.

A fresh standalone checkout cannot reproduce the sibling state. Passing local production builds does not establish a usable test baseline, and CI currently selects Go 1.25.x.

**Remediation:** Choose and pin compatible published versions or immutable commits, establish the actual minimum supported Go version, and move optional sibling development to an uncommitted/local workspace arrangement. Reconcile the integration test with a supported API without deleting its font-mapping assertions. A separate integration module is reasonable if downstream PDF/A compatibility is intentionally versioned independently.

**Acceptance:** A clean checkout builds and runs every test without sibling directories or module-file changes. The font integration test executes with an explicitly available, licensed fixture.

### R02. Required Content Failures Can Return Success

**P1 | Reproduced | Correctness, diagnostics**

Evidence: [main-flow error handling](../docflowpdf/render.go#L266), [default warning sink](../docflowpdf/render.go#L350), [element rendering](../internal/pdfrender/template_renderers.go#L50), and [test preserving warning behavior](../docflowpdf/render_test.go#L145).

A missing main template returned a nil error and an 808-byte PDF in a public-API probe. Main-flow failures are generally warnings for legacy HTML, but comparable HTML-layer failures are fatal. Multiple element and footer failures also warn and continue. Without a logger, warnings disappear; the example CLI does not install one. Main template execution also lacks `missingkey=error`, unlike i18n macros.

**Remediation:** Define one strict error policy for required content across legacy and layered inputs. Missing templates, required values, assets, and rendering failures must reach the caller. If compatibility requires best-effort rendering, make it an explicit option with collected diagnostics, never an implicit consequence of asset mode. Treat the existing warning-behavior test as a compatibility contract to migrate deliberately.

**Acceptance:** Missing main/nested/page-number templates, missing required fields, malformed typed values, and failed image rendering have tested outcomes. A CLI cannot announce success after a required section was omitted.

### R03. Failed Rendering Destroys Existing Output

**P1 | Reproduced | Data integrity**

Evidence: [RenderToFile](../docflowpdf/render.go#L113).

`os.Create` truncates the destination before input validation or rendering. Rendering invalid input over a file containing `ORIGINAL` left an empty file. The deferred `Close` error is ignored as well.

**Remediation:** Render into a temporary file in the destination directory, handle output and close errors, then replace the destination only after success. Define replacement permissions, symlink behavior, platform behavior, and whether crash durability requires syncing. Clean up temporary files on failure. Document that arbitrary `io.Writer` output cannot promise rollback after a partial write.

**Acceptance:** Invalid input, failing output, and finalization errors preserve the previous destination and leave no temporary artifacts. Successful replacement and file permissions have tests.

### R04. Monetary Rounding Produces Invalid Amounts

**P1 | Reproduced | Financial correctness**

Evidence: [FormatCurrency](../internal/format/formatter.go#L42) and [FormatFloat](../internal/format/formatter.go#L82).

The integer and fractional parts are rounded separately without carrying overflow. Rendering a currency value of `1.999` produced `CHF 1.100`, not `CHF 2.00`. `FormatFloat` follows the same pattern; with zero decimals it returns the truncated integer portion. Currency formatting also assumes two decimal places regardless of currency.

**Remediation:** Define rounding and currency-minor-unit policy first. Correct carry and zero-decimal behavior in a small patch, then adopt an exact decimal or minor-unit representation for financial values where required. Do not silently change all public float APIs in the rounding fix.

**Acceptance:** Positive/negative rounding boundaries, carry across grouping boundaries, zero decimals, negative zero, non-finite inputs, and supported zero-/three-minor-unit currencies are covered by explicit policy tests. Assert rendered monetary text, not just PDF byte presence.

### R05. Flow Sections Restart at the Same Position

**P1 | Reproduced | Layout correctness**

Evidence: [section loop](../docflowpdf/render.go#L601) and [RenderDocTemplateFlow cursor initialization](../internal/pdfrender/template_renderers.go#L59).

Each section invokes `RenderDocTemplateFlow`, which initializes a local cursor at the current page's content origin. Two ordinary sections containing `FIRST` and `SECOND` were both emitted at PDF coordinates `x=20, y=811.89`. Margin state also resets between calls.

**Remediation:** Make normal-flow cursor and pending margin state explicit and persistent across main sections. Keep page-number overlays, absolute positioning, and reusable-template rendering separate so they cannot accidentally advance normal flow.

**Acceptance:** Two and three sections advance without overlap; margins and explicit breaks behave correctly across section boundaries; page-number rendering does not change the body cursor.

### R06. Oversized Tables and Blocks Are Not Paginated

**P1 | Reproduced for tables; inspection for other blocks | Correctness, scalability**

Evidence: [flow fit checks](../internal/pdfrender/template_renderers.go#L115), [RenderTable](../internal/pdfrender/table_renderer.go#L103), and [disabled backend page breaking](../internal/pdfrender/layout.go#L75).

A valid 100-row table produced one page containing all 100 row text operations, placing later rows beyond the page. The flow layer can move a whole table to the next page but cannot split it. Text/container blocks have similar whole-block fit checks. The backend has automatic page breaking disabled.

**Remediation:** First detect and return a specific overflow error instead of silently losing visible content. Then implement row-aware pagination, repeated header rows, and an explicit policy for a row taller than one content box. Preserve header identity in the table model; current row conversion mainly reduces header status to styling. Implement text-block continuation separately.

**Acceptance:** Long tables preserve every row in order, page counts are correct, headers repeat, and rendered body bounds stay within content boxes. Oversized single rows either split under a documented policy or fail promptly, never loop or disappear.

### R07. Colspan Uses Inconsistent Column Indices

**P1 | Reproduced | Table correctness**

Evidence: [cell placement](../internal/pdfrender/table_renderer.go#L154) and [row measurement](../internal/pdfrender/table_renderer.go#L527).

Rendering sums column widths for a spanning cell but advances the cursor and the next cell index by only one column. In a three-column, 100-point-per-column table, the cell after `colspan=2` began 100 points later instead of 200. Row-height measurement also measures against a single column rather than the spanned width.

**Remediation:** Use one logical-column traversal for measurement, width inference, and rendering. Validate span values and overflow. Reuse a small shared traversal only where it removes these actual inconsistencies.

**Acceptance:** Spans followed by ordinary cells, unequal column widths, inferred columns, wrapped span content, and invalid spans all have placement and height assertions.

### R08. JSON Normalization Loses Integer Precision

**P1 | Reproduced | Data correctness, API behavior**

Evidence: [asJSONObject](../docflowpdf/render.go#L1114), [per-section normalization](../docflowpdf/render.go#L612), and [JSONSource.DecodeInto](../docflowpdf/sources.go#L67).

Source and payload values are marshaled and unmarshaled into `map[string]any`, converting JSON numbers to `float64`. An `int64(9007199254740993)` reached a template helper as `9007199254740992`. This affects identifiers as well as quantities, and repeated normalization adds allocations.

**Remediation:** Specify the normalized value contract. Use `json.Decoder.UseNumber` where JSON is required, or preserve supported native values with a deliberate conversion layer. Update numeric helpers to understand the selected representation: changing only decoding would make helpers such as `asFloat64` silently reject `json.Number`. Keep exact identifiers out of floating-point formatting.

**Acceptance:** Values above 2^53, decimals, integer boundaries, source structs/maps, JSON files, and function-map inputs retain the documented meaning through every normalization step.

### R09. The Generic Library Requires Ambient Invoice Translations

**P1 | Reproduced | Portability, structure**

Evidence: [resolveI18nInput](../docflowpdf/render.go#L384) and [translation search paths](../internal/i18n/i18n.go#L48).

With no explicit translation source, the renderer searched the working directory and ancestors for a legacy example profile. Supplying an explicit in-memory translation source allowed otherwise valid in-memory renders to proceed. Library tests inside the checkout could hide this dependency.

**Remediation:** Make no-translation rendering self-contained, with explicit generic formatting defaults. Put invoice translations in the example's configuration. Either remove ambient lookup from the library or retain it only through an explicitly named compatibility option.

**Acceptance:** Public examples render from an empty working directory with in-memory or embedded assets. Installing the library must not require the example tree, and unrelated files in parent directories must not affect output.

### R10. Asset Lookup Is Not a Security Boundary

**P1 conditional | Inspection | Security**

Evidence: [image resolution](../internal/pdfrender/text_engine.go#L800), [text/file sources](../docflowpdf/sources.go#L27), and [font discovery](../docflowpdf/render.go#L408).

Image paths can be absolute, contain parent traversal, resolve through symlinks, or fall back to the process working directory and ancestors. `AssetBaseDir` is a search preference, not confinement. Explicit font and source paths also use host filesystem access. An attacker-controlled image reference could include a readable local image in the result; this is not evidence of arbitrary-file text extraction or SSRF. No network-fetch path was identified in the reviewed resolver.

Go templates and custom functions are trusted code/configuration, not a sandbox. HTML escaping does not make filesystem access or supplied template functions safe.

**Remediation:** Document trusted-template/trusted-asset requirements immediately. Before accepting untrusted assets, add a confined resolver that opens resources relative to an authorized root or controlled filesystem and validates file types. Prefer platform/Go root-constrained opening over string-prefix checks; plain `os.DirFS` alone does not prevent symlink escapes. Restrict functions and isolate hostile rendering workloads where necessary.

**Acceptance:** Absolute paths, traversal, symlink escape, non-regular files, and denied assets have tests. Trusted local-file mode is explicit and remains separately supported.

### R11. CI Does Not Enforce the Claimed Quality Baseline

**P1 | Reproduced locally and inspected | Engineering practice**

Evidence: [workflow](../.github/workflows/maintainability.yml#L1), [file selection](../scripts/check-maintainability.sh#L28), [mapfile](../scripts/check-maintainability.sh#L51), and [boundary checks](../scripts/check-maintainability.sh#L136).

The only workflow runs a changed-file complexity/length script; it does not build, test, vet, race-test, or scan dependencies. Both changed and all-file scopes omit the public package and root Go code. Local mode examines committed history, not pending worktree changes. Boundary rules reference removed packages and suppress search errors. Function length is estimated by counting braces in text, including strings/comments, rather than parsing Go; multi-file line references use `NR` instead of `FNR`.

On this machine the script fails at `mapfile` under Bash 3.2.57. CI installs `gocyclo@latest`, so the gate's implementation is unpinned. Existing tests concentrate on selected behaviors: a passing `%PDF` header test does not detect missing or overlapping content.

**Remediation:** Add reproducible build/test/vet gates first. Make the maintainability tool portable or explicitly provision its shell; include all owned Go packages, distinguish local changes from CI diff ranges, fail on tool errors, and use Go-aware import/complexity inspection. Ratchet legacy debt rather than forcing a mass rewrite to satisfy arbitrary line budgets.

**Acceptance:** A deliberate public-package compile failure and boundary violation both fail CI. Linux and macOS gate tests cover empty diffs, pending changes, missing paths, and tool failures. Pin tooling and set least-privilege workflow permissions.

### R12. Adding CSS Layers Hides Explicit CSS Read Errors

**P2 | Reproduced | Asset correctness**

Evidence: [ResolveAssets CSS resolution](../docflowpdf/sources.go#L263).

Any failure resolving legacy CSS is ignored when `CSSLayers` is nonempty. A deliberately configured nonexistent CSS file plus one valid layer successfully resolved. The distinction between an unset source and a broken explicit source is lost; permission or I/O errors are handled the same way.

**Remediation:** Skip only an unset legacy source when layers supply CSS. Return errors for explicitly configured legacy CSS. Preserve the existing, narrower optional-layer rule that only tolerates missing optional resources.

**Acceptance:** Unset, missing, unreadable, empty, and malformed sources have separate layered/legacy tests; required source failures always identify the asset.

### R13. Resolved and Unresolved HTML Layers Infer Different Flows

**P2 | Reproduced | API consistency**

Evidence: [resolved asset defaults](../docflowpdf/render.go#L373), [AssetInput defaults](../docflowpdf/sources.go#L289), and [template-name inference](../docflowpdf/flow_defaults.go#L9).

`AssetInput.ResolveAssets` infers defaults from composed HTML, but the `RenderInput.Assets` path considers only legacy `HTML`. Valid layer-only resolved assets with an omitted flow fail to infer the main template. Inference also uses a regular expression over template text, not parsed template definitions, so it cannot reliably distinguish actual definitions from comments or other syntax.

**Remediation:** Route both APIs through one normalization/defaulting operation, preserve sequential layer parsing, and derive names from parsed template definitions once function registration requirements are accounted for. Do not revert to parsing concatenated templates: layered overrides depend on parse order.

**Acceptance:** Raw/resolved, legacy/layered, explicit/inferred flows have parity tests, including block overrides and misleading definition text in comments.

### R14. Validation Does Not Establish Stable, Immutable Inputs

**P2 | Inspection | Correctness, concurrency**

Evidence: [Flow validation](../docflowpdf/types.go#L153), [runtime validation](../docflowpdf/types.go#L213), [payload assignment](../docflowpdf/render.go#L702), [nested assignment](../docflowpdf/render.go#L1047), and [flow default mutation](../docflowpdf/flow_defaults.go#L32).

Path syntax is checked, but conflicting paths such as `a` and `a.b` are not rejected. Map iteration and overwriting intermediate objects can make conflict resolution order-dependent. Payload assignments can overwrite reserved fields or mutate a nested object referenced by another payload value. Copying `Assets` by value does not copy `MainFlow`'s backing array; filling default transformers can therefore mutate caller-owned sections and create a race when reused concurrently. These concurrency consequences were not exercised by the existing race run.

JSON flow decoding also accepts unknown keys. Numeric parsing accepts special values such as `NaN`/`Inf` in paths that only check `ParseFloat` success; page dimension checks do not comprehensively validate finiteness or usable content geometry.

**Remediation:** Define reserved names and merge precedence; reject ambiguous path prefixes and unknown configuration fields where compatible. Copy mutable structures before normalization. Validate finite, bounded dimensions and a positive content box before PDF construction.

**Acceptance:** Reusing one input concurrently is race-free and leaves it unchanged. Conflicting paths and invalid geometry produce deterministic, contextual errors. Compatibility decisions for previously tolerated unknown fields are documented.

### R15. Rendering and PDF Inspection Have No Resource Budgets

**P2; P1 conditional for hostile workloads | Inspection | Availability, scalability**

Evidence: [source reads](../docflowpdf/sources.go#L27), [template output buffer](../internal/templating/execute.go#L43), [render entry point](../docflowpdf/render.go#L129), and [unbounded Flate decompression](../internal/pdfdump/dumper.go#L206).

Source reads, template expansion, DOM traversal, image processing, document construction, and PDF inspection have no application-level byte/node/page limits. Rendering accepts no context. `RenderToWriter` constructs the complete artifact before output; it is not a bounded-memory streaming renderer. The dumper uses unbounded `io.ReadAll` on compressed streams, exposing it to decompression-memory exhaustion.

**Remediation:** Add explicit limits for input bytes, expanded template bytes, nodes/depth, rows, pages, decoded image size, and output size. Bound decompression in the dumper. Introduce cancellation at meaningful stage and loop boundaries. A deadline alone cannot interrupt arbitrary custom template functions or every backend operation; document this and use process isolation for hard resource ceilings.

**Acceptance:** Oversized inputs fail before excessive allocation where feasible; limit errors are typed/contextual; cancellation tests verify cleanup. Service adapters enforce bounded worker concurrency and backpressure rather than adding an implicit library worker pool.

### R16. Ownership Boundaries Exist but Are Not Complete

**P2 | Measured and inspected | Structure, maintainability**

Evidence: [public RenderInput](../docflowpdf/render.go#L70), [public margin option](../docflowpdf/render_options.go#L164), [layout construction](../internal/pdfrender/layout.go#L75), and [architecture document](architecture-boundaries.md#L1).

The public API exposes `templating.PageMargins` from an internal package: external consumers cannot import that type to name it normally, although scalar options and field mutation offer workarounds. The orchestration file also owns font discovery, payload conversion, template setup, and terminal-colored diagnostics. Layout construction always creates a profile-specific ring logo. Architecture documentation still lists removed domain/invoice layers and describes an example asset directory as a package.

Measured hotspots include `CellDefFromTableCell` complexity 50, `TableDefFromElement` 48, `RenderDocTemplateFlow` 38, and `buildArtifact` 30, versus a documented budget of 15. File sizes include 1,221 lines for layout and 1,131 for public rendering. These are prioritization signals, not proof that merely splitting files will improve design.

**Remediation:** Expose public configuration types, define conversion at internal boundaries, move profile assets out of generic initialization, and separate normalization, preparation, layout, and emission by responsibility. Extract cohesive behavior with characterization tests; avoid wrapper proliferation or splitting solely to satisfy line counts.

**Acceptance:** A separate consumer module can express every public configuration type. Core rendering needs no example assets. Dependency direction is checked against current packages, and refactors preserve tested output behavior.

### R17. Diagnostics Are Unstructured and Can Leak Source Details

**P2 | Inspection | Debuggability, security hygiene**

Evidence: [warning adapter](../docflowpdf/render.go#L350), [template source ordering](../docflowpdf/render.go#L646), and [i18n error annotations](../docflowpdf/render.go#L805).

Diagnostics are primarily formatted strings, often without stable stage, section, page, element, or layer identifiers. HTML source names become numeric indices, and CSS layers are concatenated, losing source provenance. I18n error strings embed source lines and ANSI color escapes in the library, which is unsuitable for machine logs and can disclose translation content or paths. A supplied `Now` controls template helpers but does not establish byte-for-byte PDF determinism; PDF metadata/order are not explicitly controlled by this package.

**Remediation:** Add structured diagnostic codes and context while preserving wrapped causes. Make content snippets opt-in/redactable. Put color and human formatting in the CLI. Offer debug-stage artifacts only through explicit options and avoid recording payloads by default. Specify whether determinism means semantic output or identical bytes before choosing a metadata policy.

**Acceptance:** Errors support `errors.Is`/`errors.As`, layer/section attribution survives preparation, default logs contain no source payloads or ANSI escapes, and reproducibility tests match the documented determinism level.

### R18. Repeated Preparation Increases Work per Section and Page

**P2 | Inspection; no performance benchmark performed | Scalability**

Evidence: [main sections](../docflowpdf/render.go#L601), [page-number rendering](../docflowpdf/render.go#L632), [template execution](../internal/templating/execute.go#L21), [stylesheet application](../internal/templating/docflow_parser.go#L25), and [measure/render text paths](../internal/pdfrender/text_engine.go#L139).

Each section and page-number operation reparses templates and CSS/selectors. Payload normalization copies data repeatedly, and measurement/rendering can build the same layout twice. Cost therefore includes repeated work proportional to sections/pages as well as document content. There are no benchmark functions in the reviewed Go sources. No throughput, latency, or peak-memory target has been established.

**Remediation:** Benchmark representative jobs before optimizing. Prepare immutable template/style structures once per appropriate scope and reuse measured layout plans where sound. Preserve per-payload locale, clock, and function-factory semantics; blindly sharing templates with captured mutable functions across renders is unsafe. Prefer per-render preparation before considering bounded cross-render caches.

**Acceptance:** Benchmarks report time, allocations, output size, and peak memory for small, medium, large, and concurrent jobs. Optimizations preserve semantic PDFs and pass race tests, with no unbounded cache growth.

### R19. The CSS Support Contract Needs Executable Examples

**P2 | Inspection | Correctness, maintainability**

Evidence: [stylesheet application](../internal/templating/docflow_parser.go#L25), [property mapping](../internal/templating/docflow_parser.go#L96), [specificity model](../internal/templating/css_selectors.go#L1), and [README limitation](../README.md#L185).

The README correctly says this is mapped-property styling, not a browser cascade. The implementation applies declarations in source order, ignores unsupported properties, and does not use the separate specificity model to resolve styles. `font-weight` and `font-style` both map to the same attribute, so later declarations can overwrite the other aspect. Complex model code can give maintainers a misleading impression of runtime support.

**Remediation:** Publish a tested support matrix for selectors, declarations, units, inline attributes, page rules, inheritance, and layer precedence. Preserve the deliberate subset unless product requirements change. Correct independent font weight/style composition and emit actionable diagnostics for unsupported constructs under an opt-in/strict policy. Either connect or remove unused models after checking their callers and purpose.

**Acceptance:** Small fixtures demonstrate actual supported behavior, including bold plus italic and layered precedence. Unsupported CSS never silently masquerades as browser-compatible rendering in documentation or tests.

### R20. Legacy and Debug Utilities Have Unclear Support Status

**P2 | Inspection | Maintainability, tooling correctness**

Evidence: [root dumper](../pdfdump.go#L1) and [internal dumper](../internal/pdfdump/dumper.go#L1).

The two PDF dump implementations duplicate substantial logic. They locate PDF objects/streams with regular expressions and are not a general PDF parser or conformance validator; arbitrary binary streams and more complex PDF structures require a real parser. Their results should not be the only correctness oracle. The three root JavaScript utilities total 3,729 lines and import numerous packages, but the repository has no package manifest/lockfile or JS test workflow. Their runtime behavior and dependency vulnerability status were not exhaustively audited here.

The invoice font and signature assets are tracked. Their presence is not itself a defect, but redistribution rights and whether samples contain sensitive material need owner confirmation.

**Remediation:** Decide which legacy entry points remain supported. Consolidate retained dumper behavior behind one implementation with writer injection, bounded decoding, tests, and a documented scope; use an established PDF parser for authoritative inspection. Archive obsolete JS tools with explicit status or give supported tools a reproducible package setup and tests. Record fixture licenses and sample-data provenance.

**Acceptance:** Each supported command has one implementation, pinned dependencies, smoke/error tests, and documentation. Examples contain only approved redistributable assets and non-sensitive data.

## Verification Performed

The working tree was clean at the start. Production code, existing tests, and committed module files were not modified during this review.

Environment: macOS/arm64, `go1.27.0`, stock Bash 3.2.57. Results using the temporary module file describe the installed sibling dependency state, not a verified release dependency graph.

| Check | Result |
| --- | --- |
| `go test ./... -count=1 -cover` | Blocked before tests: committed module requires updates. |
| Same suite with isolated `-modfile` and `-mod=mod` | Public package fails to compile at two font-extraction API calls; internal test packages pass. |
| `go build` of `./...` with isolated module file | Pass. This does not compile package tests. |
| `go vet` of `./internal/... ./cmd/...` with isolated module file | Pass. |
| `go test -race ./internal/... -count=1` with isolated module file | Pass for current tests; no public/shared-input concurrency claim. |
| `gofmt -l` on Go source directories and root dumper | No files reported. |
| Maintainability script under stock Bash | Fails: `mapfile: command not found`. |
| Direct `gocyclo -top 12 docflowpdf internal cmd` | Maximum reported complexity 50; multiple functions above 15. |
| Nine isolated expected-correctness probes | All nine fail on the behaviors recorded below. These are findings, not pre-existing suite failures. |
| Editor diagnostics | Confirms module-loading issues; also flags a redundant type assertion in the renderer. |

Per-package coverage from the isolated suite was **42.0%** for formatting, **41.8%** for PDFDOM, **29.1%** for rendering, and **51.8%** for templating. Flow adapter, i18n, dumper, and CLI packages had no own test files/0% in this invocation. Public-package coverage is unavailable because its tests did not compile. These are not an aggregate coverage score and do not measure all cross-package execution.

No dependency vulnerability scan, full public-package race run, fuzz campaign, load/heap profile, screenshot comparison, independent PDF validator run, or JavaScript test suite was performed. No absence-of-vulnerabilities or PDF/A compliance claim is made. Full security review of sibling dependency implementations is outside this assessment.

### Reproduction Cases to Preserve as Regression Tests

The temporary probes import only the public package, use small in-memory assets, and inspect simple generated PDF text streams where placement/text matters. The stream helper is diagnostic only, not a general PDF parser. Except for the deliberate ambient-i18n observation, they supply an explicit in-memory translation source.

| Case | Fixture/input | Observed result |
| --- | --- | --- |
| Missing main template | Valid assets; explicit main section names nonexistent template | Nil error, 808 PDF bytes. |
| Required CSS file | Explicit missing CSS file plus one valid CSS layer | Assets resolve without error. |
| Preserve output | Existing file contains `ORIGINAL`; invalid render input | File truncated to empty. |
| Integer precision | Source `ID=int64(9007199254740993)`; helper observes value | `9007199254740992`. |
| Layer-only resolved input | Move valid `doc` definition from `Assets.HTML` to `HTMLLayers`; omit flow | Main-flow inference error. |
| Multiple sections | Two main templates, each containing one normal div | Both at `x=20, y=811.89`. |
| Long table | 100 rows; width 200, padding 4, minimum row height 20; one 200-point column | One page, 100 row text operations. |
| Colspan | Three 100-point columns; first cell spans two, second is ordinary | Next cell advances 100 instead of 200 points. |
| Currency carry | Currency CHF, decimal `.`, value `1.999` | `CHF 1.100`. |

For the table fixtures, the public HTML syntax requires `width`, `padding`, and `row-height-min` on the table. Use named `doc` and `page-number` templates and a nonempty stylesheet, such as `@page { margin: 20pt; }`. Add focused tests to the owning packages and public integration tests once R01 is fixed; do not weaken existing integration coverage to bypass the build failure.

Temporary evidence is under `/tmp/csspdf-review.XmlX2c` on the review machine: the isolated module files, coverage output, and two probe test files. It is disposable, not a repository dependency. The table above records the durable reproduction requirements.

## Existing Strengths

- Public API, HTML/CSS parsing, PDFDOM, flow adaptation, and backend rendering already have recognizable ownership boundaries.
- Source abstractions support files, in-memory values, and `io/fs`, providing a good basis for reproducible embedded assets.
- Sequential HTML layer parsing preserves override semantics; optional-layer missing-file handling already uses wrapped error checks.
- Standard `html/template`, HTML parsing, and established CSS libraries avoid unnecessary custom parsing for much of the pipeline.
- Existing tests cover useful layering, table-sizing, locale, and font behavior, and internal tests pass under the isolated dependency setup.
- Warning hooks, a clock hook, and wrapped errors provide useful starting points for structured diagnostics and repeatable tests.

## Target Quality Contract

"Gold standard" should mean observable properties rather than a particular file length or a perfect-looking coverage number:

1. A clean checkout has a pinned, documented build and a green full suite on every supported Go version and platform.
2. Successful rendering means all required content was emitted under a documented layout/overflow policy. File errors never silently destroy prior output.
3. Values keep their declared precision and formatting semantics from input to emitted text.
4. Templates, CSS, resources, and custom functions have explicit trust and support contracts; hostile inputs are rejected or isolated within resource budgets.
5. Public input reuse is immutable/race-tested, output is independent of ambient example directories, and all public configuration types are usable by external modules.
6. Errors identify stage and location without leaking payloads. Semantic or byte determinism is specified and tested, not inferred from the clock hook.
7. Performance limits come from representative benchmarks and profiles; optimizations preserve measured behavior.
8. Every supported component has a clear owner, reproducible dependencies, tests, and current documentation.

## Phased Remediation

Use small pull requests. Each row is a bounded Copilot work item, not permission to combine the whole phase into one edit. Address phases 0-2 before a correctness-sensitive release. Complete phase 3 before exposing rendering to untrusted assets or service workloads. Do not postpone the trust warning or CI build/test gates until the architecture work.

### Phase 0: Reproducible Baseline

**Goal:** Make failures trustworthy and independently reproducible. **Covers:** R01, the immediate portions of R10/R11/R20.

| Work item | Scope and dependencies | Completion gate |
| --- | --- | --- |
| 0A Dependency/toolchain contract | Module files and downstream integration API. Owner selects supported versions and minimum Go version. | Clean clone without siblings: build, full tests, and tidy produce no tracked changes. |
| 0B Hermetic integration fixtures | Font test and fixture metadata; retain Type0/ToUnicode/character assertions. Depends on 0A. | Test executes, not skips, on clean Linux/macOS runners using approved fixtures. |
| 0C Essential CI gates | Workflow only plus a small shared check entry point. Pin actions/tools; least-privilege permissions. Depends on 0A. | Build, tests, vet, formatting, and dependency checks run for public and internal code; full-suite failures block merge. |
| 0D Immediate support/trust documentation | README and security/support documentation only. | State trusted-input assumptions, non-browser CSS scope, supported toolchains, and provisional legacy-command status. |

**Exit:** No release can rely on a local sibling checkout or bypass an uncompiled integration test. Record the initial vulnerability scan and triage its actual results; do not equate dependency freshness with security.

### Phase 1: Reliable Inputs, Errors, and Output

**Goal:** Eliminate silent corruption and make failure behavior consistent. **Covers:** R02-R04, R08-R09, R12-R14.

| Work item | Scope and dependencies | Completion gate |
| --- | --- | --- |
| 1A Required-content error policy | Public orchestrator, flow renderer, focused tests; decide strict default versus explicit legacy compatibility first. | Missing templates/required values/images fail visibly; layered and legacy cases agree; CLI exit status is tested. |
| 1B Atomic file output | File-output helper and tests only; do not refactor layout. | Existing destination preserved for render/write/close failure; temp cleanup and successful replacement tested. |
| 1C Rounding correction | Formatter and monetary integration tests. | Carry, negative values, and zero-decimal cases pass; publish rounding policy. Exact-decimal API evolution is a separate follow-up ticket. |
| 1D Numeric normalization | Source normalization plus numeric helpers and tests. | Large integer probe passes through every input mode; `json.Number` or native-value semantics work with default/custom helpers. |
| 1E Self-contained defaults | i18n initialization and example configuration. | In-memory render works in an empty directory; formatting separators have generic defaults. |
| 1F Required CSS errors | Asset resolution and source tests only. | Explicit read failures are preserved; unset/optional source behavior remains supported. |
| 1G HTML flow parity | Default inference and asset normalization, with layered tests. | Layer-only resolved/unresolved inputs agree and sequential block overrides remain intact. |
| 1H Immutable validated configuration | Flow copy/validation helpers and focused tests. | Reserved/conflicting paths rejected; shared input unchanged after rendering; concurrent normalization passes `-race`. |

**Compatibility control:** Changing error defaults, numeric types, unknown-field handling, and rounding semantics must have migration notes and release-version decisions. Do not hide semantic changes inside extraction/refactoring commits.

### Phase 2: Layout Correctness

**Goal:** Establish layout invariants before changing architecture. **Covers:** R05-R07 and geometry from R14.

| Work item | Scope and dependencies | Completion gate |
| --- | --- | --- |
| 2A Persistent flow state | Flow state and section rendering only. | Multiple sections do not overlap; page overlays and absolute elements do not corrupt normal flow. |
| 2B Span-aware column traversal | Table measurement/rendering and tests only. | Colspan positions/heights agree for fixed and inferred columns. |
| 2C Geometry and overflow detection | Finite content-box validation and whole-block fit checks. | NaN/Inf, impossible margins, and oversized blocks produce bounded, contextual failures. |
| 2D Table row pagination | Row model/header identity, page placement, renderer tests. Depends on 2A-2C. | 100-row and mixed-height fixtures preserve order, repeat headers, and remain in bounds; oversized-row policy is tested. |
| 2E Text continuation | Text layout/fragmentation, independent of table algorithms. Depends on 2A/2C. | Long text spans pages without loss/duplication; line spacing and margins remain correct. |
| 2F Semantic PDF regressions | Shared test fixtures and an independent PDF reader/validator. | Page counts, text, font mappings, and placement pass; selected raster snapshots reviewed for visual layout. |

**Exit:** All nine reproduced defect cases are permanent passing regressions. Layout tests include explicit breaks, first/subsequent page margins, running footers, empty sections, UTF-8, and long content. Keep PDF structural/semantic assertions primary; byte snapshots alone are sensitive to metadata.

### Phase 3: Security and Operational Boundaries

**Goal:** Make deployment assumptions enforceable. **Covers:** R10, R14-R15, security aspects of R17/R20.

| Work item | Scope and dependencies | Completion gate |
| --- | --- | --- |
| 3A Confined resource resolver | Resource interface and file/FS implementation, then image/font adapters in separate patches. | Traversal, absolute path, symlink, and file-type tests pass; explicit trusted-file mode remains available. |
| 3B Input/expansion limits | Source reads and template output first; node/row/page/image/output limits in subsequent small patches. | Limit-boundary and over-limit tests fail predictably with cleanup; no silent truncation. |
| 3C Context-aware rendering | Additive context API and stage/loop checks; old APIs delegate using a background context. | Cancellation before preparation, during long layout, and before emission is tested; non-interruptible custom code limitations documented. |
| 3D Safe PDF inspection | Bound stream decompression and file size; inject output writer. | Compressed-bomb fixture stops at configured limit; errors are returned rather than swallowed. |
| 3E Security regression gate | Fuzz supported parser/normalization entry points with limits; pin vulnerability scanning. | Seed corpus, time-bounded fuzz smoke, regular longer runs, and vulnerability exception policy are present. |

**Exit:** Threat model separates trusted templates/functions from untrusted data/assets. Service integrations document worker limits, maximum document sizes, and isolation requirements. No claim of full sandboxing is made merely because a context or resolver was added.

### Phase 4: Ownership and Diagnostics

**Goal:** Reduce change coupling without changing established behavior. **Covers:** R11, R16-R17, R19-R20.

| Work item | Scope and dependencies | Completion gate |
| --- | --- | --- |
| 4A Public configuration types | Public margin/settings types and internal conversion; add an external-consumer compile fixture. | No public signature requires naming an internal type; documented compatibility maintained. |
| 4B Orchestration decomposition | One extraction per PR: normalized input, fonts/resources, payload transformation, then diagnostic formatting. | Characterization tests unchanged; no backend/framework replacement. |
| 4C Layout ownership | Separate flow state, table conversion, reusable templates, and optional profile graphics in independent PRs. | Generic render initializes no invoice assets; output regressions pass. |
| 4D Structured diagnostics | Diagnostic types/codes, provenance, redaction, CLI formatter in staged changes. | Stable stage/section/page/layer context; wrapped causes; no ANSI or payload snippets in default library errors. |
| 4E CSS support contract | Tested support matrix, independent font-style composition, removal/integration of unused models after usage checks. | Supported subset examples execute; unsupported constructs have documented diagnostics. |
| 4F Tooling and legacy consolidation | Portable Go-aware gates, current boundary docs, one dumper, explicit JS support decisions. | Gates test dirty/empty/all scopes; supported commands have smoke tests and reproducible dependencies. |

**Exit:** Architecture documents match actual packages. Apply a complexity ratchet to touched functions; isolate table-driven mappings from genuinely tangled control flow. A waiver must name the reason, owner, and review point rather than silently excluding the public package.

### Phase 5: Measured Performance and Determinism

**Goal:** Optimize observed bottlenecks under a stable correctness contract. **Covers:** R18 and determinism from R17.

| Work item | Scope and dependencies | Completion gate |
| --- | --- | --- |
| 5A Benchmark corpus | Small letter, layered multi-section document, long table, images, UTF-8, and concurrent independent jobs. | Baseline latency, allocations, heap/peak RSS, pages, and output sizes recorded with machine/toolchain details. |
| 5B Prepared inputs | Per-render compiled templates/styles and normalized data; preserve context-dependent function behavior. | Fewer repeated parse/serde operations; semantic output unchanged; race tests pass. |
| 5C Reusable layout plans | Remove demonstrated duplicate measurement work only where profiles support it. | Benchmark improvement is measurable without pagination or font regressions. |
| 5D Determinism contract | Fixed clock plus explicit metadata/order policy if byte reproducibility is required. | Repeated/process-separated renders satisfy the chosen semantic or byte-level contract. |

**Exit:** Publish a supported workload envelope and agreed regression budget on a reference runner. Do not set arbitrary latency promises before benchmarking or add a global cache without a capacity/eviction policy.

### Phase 6: Release Readiness and Maintenance

**Goal:** Keep quality from regressing. **Covers:** All findings through sustained gates.

- Run the full supported Go/OS matrix, integration fixtures, semantic PDF validation, race tests, and scheduled fuzz/security jobs.
- Require tests for each fixed behavior and risk-weighted coverage on new logic. Ratchet package coverage from the recorded baseline; do not pursue 100% line coverage by weakening assertions or testing only wrappers.
- Verify README examples in an external consumer module and from a clean working directory.
- Document API compatibility, deprecations, supported CSS/HTML, resource policy, troubleshooting, and release procedure.
- Confirm font/signature/sample-data provenance and the support status of every shipped utility.
- Close every P1 and applicable conditional P1 with a linked regression test. Record remaining P2 items with an owner, rationale, and milestone; do not label the project gold-standard solely because lint gates are green.

## Copilot Work Item Protocol

Use the following task template for each row above. The human owner approves policy changes; Copilot implements the bounded mechanics and supplies evidence.

```text
Task: <phase/item and finding IDs>
Goal: <one observable outcome>
Allowed scope: <owning files/packages and focused tests>
Preconditions: <dependency baseline and completed predecessor items>
Compatibility decision: <approved behavior, or stop and ask>
Non-goals: <no unrelated cleanup, no new framework, no broad API rewrite>

1. Read the owning behavior and nearest test/call site.
2. State a local hypothesis and create a failing regression test.
3. Confirm it fails for the intended reason, not a dependency/setup error.
4. Implement the smallest change; immediately rerun that check.
5. Run affected-package tests, then required repository gates.
6. Check callers, input immutability, error context, and relevant edge cases.
7. Report changed behavior, exact commands/results, and unresolved risks.

Done means:
- The regression passes without deleting or weakening existing assertions.
- Required gates pass, or a pre-existing blocker is explicitly reported.
- No unrelated edits or generated artifacts are included.
- Public behavior and migration notes match the approved policy.
```

Keep each PR to one behavioral contract, typically one implementation area and its tests. Split larger pagination, resource-resolution, and diagnostic work at tested boundaries. Do not bundle dependency upgrades, mechanical extraction, and behavior changes. Human review should focus on the assertions and compatibility decisions, not just whether generated code compiles.

## Decisions Needed From the Maintainer

1. Is this library for trusted local authoring only, or will it process tenant/user-supplied templates and assets?
2. Which Go version and immutable sibling-library revisions are the release baseline?
3. Should strict rendering become the default immediately, or through a documented compatibility transition?
4. What are the monetary rounding/minor-unit rules and required exact-number types?
5. Which page-overflow behaviors are supported, and what maximum document size/concurrency must be supported?
6. Is reproducibility semantic or byte-for-byte, and are PDF/A or accessibility requirements part of the product contract?
7. Are the root JavaScript tools and duplicate dumper public interfaces, and are all sample assets approved for redistribution?

These decisions constrain implementation; they do not prevent starting Phase 0 and adding the verified regression cases.