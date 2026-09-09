# Code Quality Review and Remediation Plan

Review date: 2026-09-08. Baseline commit: `ce1ff17` on `main`.

This review supersedes the 2026-09-07 assessment. That assessment's baseline
commit no longer exists because repository history was rewritten to remove
private data and legacy example assets. Its twenty findings (R01-R20) were
remediated in Phases 0-6; their disposition is recorded below so the closed
items stay traceable without depending on rewritten commit identifiers.

## Recommendation

The project has moved from "not yet a reliable library" to "reliable with a
maintained quality gate." Every previously identified release blocker is closed
with regression tests, the build is hermetic on Go 1.26.6 and 1.27.x, the
public API is consumable from an external module, and the quality gate covers
build, vet, format, tests, coverage floors, race, fuzz, and vulnerability
scanning.

Phases 7 through 10 closed the static-analysis, dead-code, error-taxonomy,
panic, legacy-policy, core-test-depth, complexity-headroom, and backend-module
findings. The maintainability gate now enforces complexity 12 and 550-line
production-file limits across the first-party tree.

This report covers code quality, structure, maintainability, test coverage,
and error handling. It is an engineering assessment, not a security
certification, legal assessment, or PDF conformance audit.

## Disposition of Prior Findings

| ID | Finding | Status | Evidence |
| --- | --- | --- | --- |
| R01 | Builds depended on unpinned sibling checkouts | Closed | `go.mod` pins all dependencies; backend is repository-owned in `third_party/gofpdf`; clean-checkout and read-only container validation in `docs/phase0-baseline.md` |
| R02 | Required-content failures returned success | Closed | Strict rendering is the default; legacy behavior only via `WithLegacyPartialRendering`; `docs/phase1-migration.md` |
| R03 | Failed rendering destroyed existing output | Closed | Temp-file-and-rename in `output.go` with cleanup tests |
| R04 | Monetary rounding produced invalid amounts | Closed | Carry-correct formatting with policy tests in `internal/format` |
| R05 | Flow sections restarted at the same position | Closed | Persistent `docFlowState`; multi-section tests in `internal/pdfrender` |
| R06 | Oversized tables/blocks not paginated | Closed | Row-aware pagination with repeated headers (`table_pagination.go`); text continuation; oversized-row policy tested |
| R07 | Colspan index inconsistency | Closed | Single logical-column traversal in `table_layout.go` with span tests |
| R08 | JSON normalization lost integer precision | Closed | `json.Number` normalization with helper support and boundary tests |
| R09 | Library required ambient invoice translations | Closed | Self-contained defaults; ambient lookup removed; external-consumer render from empty directory |
| R10 | Asset lookup was not a security boundary | Closed | `ConfinedFileResolver` on `os.Root`; explicit `TrustedFileResolver`; `docs/phase3-security.md` |
| R11 | CI did not enforce the quality baseline | Closed | Matrix quality/race/security workflows; portable Bash 3.2 gates; pinned tools |
| R12 | CSS layers hid explicit CSS read errors | Closed | Unset versus broken source distinction with tests in `sources_test.go` |
| R13 | Resolved/unresolved HTML layers inferred different flows | Closed | Shared normalization path; parity tests |
| R14 | Validation did not establish immutable inputs | Closed | Flow cloning, reserved-path rejection, concurrent-reuse race tests |
| R15 | No resource budgets | Closed | `RenderLimits`, context-aware entry points, bounded dumper decompression |
| R16 | Incomplete ownership boundaries | Closed | Public `PageMargins`; orchestration decomposed; profile assets out of generic layout; boundary checks current |
| R17 | Unstructured diagnostics | Closed | `DiagnosticError` with codes/stages; no ANSI in library errors; CLI formatting separated |
| R18 | Repeated preparation per section/page | Closed | `PreparedFlow`/`PreparedTemplates`/`PreparedStylesheet`; benchmark corpus and budget in `docs/phase5-performance.md` |
| R19 | CSS support contract lacked executable examples | Closed | `AnalyzeCSSSupport`, independent font weight/style composition, fixture tests |
| R20 | Legacy/debug utilities had unclear status | Closed | Single `internal/pdfdump` implementation; legacy JavaScript utilities removed from tree and history; `docs/provenance.md` |

Determinism (R17/R18 follow-up) is byte-level for fixed-clock renders and is
tested in-process and cross-process.

## Method

All measurements were taken on the baseline commit with a clean worktree
(only the ignored `output/` directory untracked) using Go 1.26.6 on
macOS/arm64. Commands and results:

| Check | Result |
| --- | --- |
| `scripts/check-quality.sh` | Pass: manifests, format, build, vet, tests, coverage floors, backend tests, `govulncheck` (no vulnerabilities) |
| `go test -race ./... -count=1` | Pass |
| `CHECK_SCOPE=all scripts/check-maintainability.sh` | Pass: no function above complexity 15, no file above 600 lines, no function above 80 lines, boundaries intact |
| `scripts/check-layered-rollout.sh` | Pass: layered example renders |
| `gocyclo -avg` (first-party, non-test) | Average 3.9; 40 functions above 10; 4 functions at exactly 15; none above 15 |
| `go test -covermode=atomic -coverprofile` | 58.8% of statements overall; per-package figures below |
| `go vet ./...` | Pass |
| Dead-code and error-pattern surveys | `rg`-based; results cited per finding. No `TODO`/`FIXME` markers in first-party code |

No third-party static analyzer beyond `go vet` is installed or gated, which
is itself a finding (N09).

## Current Metrics

Source and test volume by package (non-test lines / test lines) and statement
coverage at the baseline commit:

| Package | Source | Tests | Coverage | Floor | Functions below 50% |
| --- | ---: | ---: | ---: | ---: | ---: |
| `cmd/pdfdump` | 26 | 30 | 66.7% | 66.7 | 1 |
| `cmd/dom-parse` | 113 | 45 | 74.5% | 74.5 | 1 |
| `cmd/gen-example` | 179 | 44 | 48.0% | 25.2 | 4 |
| `csspdf` | 3,475 | 3,109 | 73.1% | 73.1 | 41 |
| `internal/flowrender` | 266 | 76 | 64.4% | 64.4 | 9 |
| `internal/format` | 210 | 98 | 57.7% | 56.5 | 6 |
| `internal/i18n` | 165 | 56 | 88.2% | 88.2 | 0 |
| `internal/pdfdom` | 1,668 | 505 | 48.4% | 48.4 | 104 |
| `internal/pdfdump` | 1,237 | 93 | 27.1% | 25.4 | 38 |
| `internal/pdfrender` | 4,269 | 1,035 | 63.3% | 63.3 | 53 |
| `internal/templating` | 784 | 220 | 45.5% | 45.5 | 22 |
| **Total** | **12,392** | **5,311** | **58.8%** | | **279** |

Largest source files: `internal/pdfrender/text_engine.go` (590),
`internal/pdfdom/html_parser.go` (578), `internal/pdfdom/elements.go` (557),
`internal/pdfrender/layout.go` (527), `internal/pdfrender/table_renderer.go`
(525). Largest test file: `render_test.go` (1,282).

Error construction sites (`fmt.Errorf`/`errors.New`) and how many wrap a
cause with `%w`: `csspdf` 93/47, `internal/pdfrender` 69/15,
`internal/pdfdom` 35/4, `internal/templating` 16/9, `internal/pdfdump` 11/4,
`internal/i18n` 6/1. Typed error kinds: `DiagnosticError`, `BudgetError`,
`LimitError`, `ComplexityLimitError`, `OutputLimitError`,
`InspectionLimitError`, `PageLimitError`, plus two unexported kinds.

## Findings

Severity meanings:

- **P2:** Important maintainability, test-depth, or robustness gap to close
  before the next minor release.
- **P3:** Hygiene item with low risk; batch into a related P2 work item.

There are no open P1 findings. Evidence labels distinguish **measured**
results from **inspection**.

### N01. Core Package Coverage Lags the Public Facade

**P2 | Measured | Test coverage, maintainability**

Evidence: coverage table above; `go tool cover -func` output.

The public facade is well covered (73.1%) but the packages that do the
actual parsing and layout are not: `internal/pdfdom` has 104 functions below
50% coverage, `internal/pdfrender` 53, `internal/templating` 22. Test volume
follows the same pattern: `pdfrender` has 4.1 source lines per test line,
`pdfdom` 3.3, `pdfdump` 13.3, versus `csspdf` at 1.1. Most core-package
behavior is exercised only indirectly through facade integration tests, so
failures surface as end-to-end PDF differences rather than as unit assertions
that name the broken rule.

Specific uncovered surfaces: `internal/templating/docflow_parser.go` exports
(`ParseStyledFragment`, `CaptureAttrNames`, `ApplyCSSDeclaration`,
`SetOrReplaceAttr`, `CollectText`, `ParseBorderShorthand`) are 0% in their
own package; `internal/pdfdom/span_style.go` attribute handling is untested;
all `internal/pdfdump` PNG-predictor and dictionary-tokenizer functions are
0%.

Public option constructors in `render_options.go` (twenty-plus
`With*` functions and `RenderContext`) are 0% covered. They are trivial, but
they are the public contract; a single table test would prevent a
misassigned field from shipping.

`render_test.go` at 1,282 lines and `sources_test.go` at 720
lines mix unrelated concerns, which discourages adding focused cases.

**Remediation:** Add package-level characterization tests for the listed
exported functions and for `span_style.go` attribute parsing. Add a
table-driven option test that asserts each `With*` sets exactly its field.
Split `render_test.go` by concern (determinism, layering, limits, i18n,
fonts). Raise floors as coverage lands; do not raise floors first.

**Acceptance:** `internal/pdfdom` at or above 65%, `internal/pdfrender` 72%,
`internal/templating` 60%, `internal/pdfdump` 50%, overall at or above 68%,
with floors updated to the new observed minimums across the CI matrix. No
public test file exceeds 600 lines.

### N02. Dead and Deprecated Code Is Retained

**P2 | Measured | Maintainability**

Evidence: caller counts via `rg` on non-test code.

Functions with zero non-test callers: `csspdf.buildArtifact`,
`csspdf.resolveRenderAssets`, `csspdf.resolveLegacyCSS`,
`templating.ExecuteNamed`, `templating.ExecuteNamedFromSources`,
`templating.ApplyStylesheet`. Deprecated `pdfdom.ParseHTMLIntroElem`,
`htmlBuildIntroDiv`, `flowrender.BuildNamedElements`, and
`flowrender.BuildNamedElementsWithFuncs` have zero callers, so the
deprecations were never acted on. `RenderInput.WarningWriter` is deprecated
but names no removal release. `template_renderers.go` contains
`_ = bottomMargin`, a computed value that is never used, and two dead stores
to `yPos`.

Unreferenced code inflates coverage denominators, keeps deprecated surfaces
alive, and hides whether the replacement API is complete.

**Remediation:** Delete unexported zero-caller functions and the zero-caller
deprecated wrappers. Remove the unused `bottomMargin` computation and dead
stores. Schedule removal of `WarningWriter` for a named release in
`docs/api-compatibility.md`, since it is public.

**Acceptance:** A pinned static analyzer (N09) reports no unused code.
`docs/api-compatibility.md` lists a removal release for every remaining
deprecated symbol.

### N03. Error Taxonomy Is Fragmented and Wrapping Is Uneven

**P2 | Measured and inspected | Error handling, debuggability**

Evidence: typed-error survey; wrap ratios in Current Metrics;
`diagnostics.go`, `limits.go`, `resource_resolver.go`,
`internal/flowrender/complexity.go`, `internal/templating/execute.go`,
`internal/pdfdump/bounded_output.go`, `internal/pdfrender/flow_state.go`.

Seven distinct limit-style error types exist across five packages
(`BudgetError`, `LimitError`, `ComplexityLimitError`, `OutputLimitError`,
`InspectionLimitError`, `PageLimitError`, and the resolver's `LimitError`).
A caller that wants "was a budget exceeded" must `errors.As` against several
types or rely on `DiagnosticCode` mapping at the facade. The facade
`DiagnosticError` is a good boundary, but the internal kinds do not share a
marker interface or sentinel.

Wrapping is inconsistent: `internal/pdfdom` wraps 4 of 35 constructions,
`internal/pdfrender` 15 of 69, `internal/i18n` 1 of 6. Many unwrapped sites
are legitimate leaf errors, but several discard an underlying parse or I/O
cause, which loses `errors.Is(err, fs.ErrNotExist)`-style checks and stage
context.

**Remediation:** Introduce one internal `LimitExceeded` marker (interface or
sentinel) that every limit type satisfies, and assert it at the facade.
Audit unwrapped constructions in `pdfdom`, `pdfrender`, and `i18n`; wrap
where a cause exists, leave true leaves alone. Add a lint rule for
`fmt.Errorf` with an `err` argument but no `%w`.

**Acceptance:** `errors.Is(err, csspdf.ErrLimitExceeded)` (or
equivalent) is true for every limit failure and is tested per limit. Wrap
ratio in `pdfdom` and `pdfrender` reflects an explicit leaf-versus-wrapped
classification recorded in the PR.

### N04. Node Builders Panic on Invalid Children

**P2 | Inspection | Error handling, robustness**

Evidence: `internal/pdfdom/elements.go` `Add`/`AddLine` call `panic(err)` on
`validateChild` failure; 33 `.Add(`/`.AddLine(` call sites in non-test
library code.

The builder API panics inside library code when an element receives a child
type it does not accept. The public facade has no `recover` boundary, so a
parser bug or an unexpected HTML structure that reaches these builders will
crash the caller's process rather than return a `DiagnosticError`. This is
inconsistent with the strict-but-recoverable error policy established in
Phase 1.

**Remediation:** Either (a) change `Add`/`AddLine` to return an error and
update the 33 call sites, or (b) keep the fluent builder for internal
construction and add a single `recover` at the facade that converts a
builder invariant violation into a `DiagnosticError` with
`DiagnosticInvalidInput`. Option (a) is preferred; option (b) is acceptable
if the builders are treated as internal invariants and the recovered panic
is logged with stage context.

**Acceptance:** A test that feeds an invalid child through the public API
receives an error, not a panic. No `panic(` remains in first-party library
packages outside documented invariant checks.

### N05. Span Attribute Parsing Swallows Errors

**P3 | Inspection | Error handling, correctness**

Evidence: `internal/pdfdom/span_style.go` discards the error from
`strconv.ParseFloat` for `font-size` and from `ParseLengthValue` for
`border-width`.

An invalid `font-size="abc"` silently yields size 0, which then falls
through to defaults or renders invisibly. Every other typed-value path in the
project fails under strict rendering. This is a small but real inconsistency
in the required-content policy.

**Remediation:** Return the parse error and let strict mode reject it;
under `WithLegacyPartialRendering`, emit a warning. Add tests for invalid,
empty, and unit-suffixed values.

**Acceptance:** Invalid span numeric attributes fail in strict mode with the
attribute name in the error.

### N06. The Complexity Ratchet Has No Headroom

**P2 | Measured | Maintainability**

**Status: Closed in Phase 10.** The four named functions are now at complexity
4, 5, 9, and 6 respectively. No first-party function exceeds 12, the largest
production file is 545 lines, and the default gate limits are 12 and 550.

Evidence: `gocyclo` top list; `scripts/check-maintainability.sh` budgets.

Four functions sit exactly at the complexity ceiling of 15:
`templating.parsePageRuleDeclarations`, `pdfrender.normalizeTableFontStyle`,
`(*LayoutPDF).RenderUseTemplateElement`, and `pdfdump.(*textArrayState).consume`.
Ten more are at 13-14. Three files are within 50 lines of the 600-line file
budget. Any bug fix touching those functions or files will trip the gate and
tempt a waiver or a mechanical split that does not improve design.

The average complexity of 3.9 is healthy; this is a local headroom problem,
not a systemic one.

**Remediation:** Refactor the four at-ceiling functions behind
characterization tests, targeting 10 or below, one PR each. Prefer extracting
a cohesive helper or a table-driven mapping over splitting arbitrarily. For
the three near-budget files, move a cohesive group of functions to a sibling
file only where the group has a name that is not "misc."

**Acceptance:** No first-party function above 12; no file above 550 lines.
Then lower `MAX_CYCLO` to 12 for touched functions in the changed-scope
gate so the ratchet keeps tightening.

### N07. Nested Backend Module Declares Go 1.12 Language Semantics

**P2 | Inspection | Correctness, maintainability**

**Status: Closed in Phase 10.** The nested module declares Go 1.22 and no
longer has a self-replace directive. All 126 range loops were reviewed with no
goroutine or deferred closure capture; nested vet, tests, and race tests pass.

Evidence: `third_party/gofpdf/go.mod` declares `go 1.12` and contains
`replace gofpdf => ./`; 126 `range` loops in the package.

The root module requires Go 1.26.6, but the repository-owned backend compiles
under Go 1.12 language semantics. This means pre-1.22 shared loop-variable
semantics, no range-over-integer, and older vet analyzers for that package.
No goroutine closures over loop variables were found, so no current bug is
claimed, but the semantic split is invisible to contributors and the nested
module's vet/format checks run with an older ruleset than the rest of the
repository. The `replace gofpdf => ./` directive is a non-module path and
appears vestigial.

**Remediation:** Raise the nested directive to at least `go 1.22` (or align
with the root minimum), run the nested test suite and `go vet`, review any
loop-variable capture the compiler flags, and remove the vestigial replace
directive. Record the change in `third_party/gofpdf/PATCHES.md`.

**Acceptance:** Both modules declare a language version at or above 1.22;
nested tests, vet, and the root quality gate pass; `PATCHES.md` records the
directive change and any semantic review findings.

### N08. Coverage Floors Lag Observed Values

**P3 | Measured | Engineering practice**

Evidence: `scripts/coverage-baseline.txt` versus the coverage table.

`cmd/gen-example` floor is 25.2% while observed coverage is 48.0% after the
invoice command was removed; `internal/format` 56.5% versus 57.7%;
`internal/pdfdump` 25.4% versus 27.1%. The ratchet therefore permits a
regression of up to 23 points in `gen-example` without failing.

**Remediation:** Re-record floors as the minimum across the current CI
matrix after N01 lands, per the existing rule in
`docs/phase6-release-readiness.md`.

**Acceptance:** Every floor is within two points of the matrix minimum.

### N09. Static Analysis Is Limited to `go vet`

**P2 | Inspection | Engineering practice**

Evidence: `scripts/check-quality.sh`; no `staticcheck` or `golangci-lint`
present or pinned.

The dead code in N02, the discarded errors in N05, and the unused variable
in `template_renderers.go` are all standard `staticcheck`/`unused`/`errcheck`
findings. The current gate would not catch a reintroduction.

**Remediation:** Add a pinned `staticcheck` (or `golangci-lint` with
`unused`, `errcheck`, `staticcheck`, `gosimple`) invocation to
`scripts/check-quality.sh` and the CI quality job. Start with the default
rule set; add exclusions only with a comment naming the reason.

**Acceptance:** The analyzer runs in CI on every supported Go version, is
version-pinned, and passes on the baseline after N02/N05 are fixed.

### N10. PDF Inspection Tool Has Broad Surface and Two Entry Points

**P2 | Measured | Maintainability, test coverage**

Evidence: `internal/pdfdump` 1,237 lines at 27.1%; `cmd/pdfdump` and
`cmd/gen-example dump-pdf` both call `pdfdump.DumpPDF`.

The dumper is documented as a supported bounded diagnostic tool, but its
xref/PNG-predictor decoder, dictionary tokenizer, stream formatter, and ANSI
styling are untested. It is also reachable from two CLIs with identical
behavior, and `runDumpPDF` in `gen-example` is 0% covered. After the invoice
command's removal, `gen-example` exposes only `layered` and `dump-pdf`, so its
name no longer describes it well.

**Remediation:** Add golden-file tests for the tokenizer, predictor decoder,
and stream formatting using small synthetic PDFs (pdfcpu can generate them).
Remove `dump-pdf` from `gen-example` in favor of the root `pdfdump` command,
or fold both into one `cmd/csspdf` tool with `render-example` and `dump`
subcommands; record the decision in `docs/api-compatibility.md`.

**Acceptance:** One dump entry point; `internal/pdfdump` at or above 50%;
CLI smoke tests cover usage, missing-file, and over-limit paths.

### N11. Legacy Partial-Render Path Has No Removal Date

**P3 | Inspection | Maintainability**

Evidence: `internal/pdfrender/render_errors.go` `recoverableRenderError`
used at nine sites; `WithLegacyPartialRendering` and deprecated
`WarningWriter` retained; `docs/api-compatibility.md` says "no removal
release scheduled."

The compatibility path is correctly opt-in, but every renderer change must
consider two error policies. Keeping it indefinitely doubles the test matrix
for element rendering.

**Remediation:** Owner decides a removal release. Until then, ensure each
`recoverableRenderError` site has a test in both modes so the legacy path
does not silently diverge.

**Acceptance:** A removal release is named, or a documented rationale for
indefinite support is recorded with the test-both-modes rule enforced.

### N12. Documentation References Rewritten-Away Commits

**P3 | Measured | Documentation**

Evidence: `docs/phase5-performance.md` cited the end-of-Phase-4 commit by
abbreviated hash; that hash no longer exists after the history rewrite.

Phase evidence documents that cite commit identifiers become unverifiable
after any history rewrite. Content-based references (file paths, tags, or
reproducible commands) survive rewrites.

**Remediation:** Replace the stale identifier with the commit subject and
a reproducible command. Adopt a rule that evidence documents cite tags or
file content, not raw hashes, unless the hash is on a protected branch that
will never be rewritten.

**Acceptance:** No document references a commit that `git cat-file -e`
cannot resolve; the release procedure notes the citation rule.

## Existing Strengths

- All twenty prior findings, including nine reproduced document-correctness
  defects, are closed with named regression tests.
- The quality gate is portable (Bash 3.2), pinned, least-privilege, and
  enforced across a Linux/macOS/Windows amd64 and Go 1.26.6/1.27.x matrix.
- The public facade has a coherent option API, typed diagnostics with
  stable codes, resource budgets, cancellation, confined file access, and
  byte-level determinism under a fixed clock.
- Average cyclomatic complexity is 3.9 with no function above the budget and
  no `TODO`/`FIXME` debt markers.
- Architecture boundaries are documented and mechanically checked; internal
  packages do not import the facade.
- Dependencies are current and `govulncheck` is clean on the minimum
  toolchain.
- Provenance and support status are documented; private data and
  unapproved assets have been removed from history.

## Target Quality Contract

The Phase 0-6 contract remains in force. This review adds:

1. Core packages have direct unit coverage proportional to their source
   volume; integration tests confirm composition, not basic behavior.
2. Library code never panics on input; every failure is a typed, wrapped
   error reachable with `errors.Is`/`errors.As`.
3. Static analysis beyond `go vet` runs in CI and passes with documented
   exclusions only.
4. The complexity and file-length ratchets keep tightening; waivers name an
   owner and review date.
5. Both modules compile under a current language version.
6. Evidence documents cite durable references.

## Phased Remediation

Use small pull requests. Each row is one bounded work item. Phases 7 through 9
are complete. Phase 10 builds on Phase 9's characterization tests.

### Phase 7: Static Analysis and Hygiene (completed 2026-09-08)

**Goal:** Catch regressions of the hygiene findings mechanically.
**Covers:** N02, N08, N09, N12.

| Work item | Scope and dependencies | Completion gate | Result |
| --- | --- | --- | --- |
| 7A Pinned analyzer | `scripts/check-quality.sh`, CI quality job. | `staticcheck` (or `golangci-lint`) runs on every matrix cell and blocks merge. | `staticcheck` v0.8.1 (default checks) and `errcheck` v1.20.0 run on the root module; `staticcheck -checks 'SA*'` runs on `third_party/gofpdf`. Exclusions live in `scripts/errcheck-excludes.txt` with reasons. A probe with an unused function and a discarded error fails both tools. |
| 7B Dead code removal | Six zero-caller functions, `_ = bottomMargin`, deprecated `pdfdom` functions. | Analyzer clean; tests unchanged. | Fourteen unused symbols removed across seven packages, including the duplicate `hexToRGB` parser, which now delegates to the tested table parser. Two dead stores fixed. Six ignored `fmt.Sscanf` results replaced by explicit scanners with identical semantics. Backend SA findings fixed and recorded in `third_party/gofpdf/PATCHES.md`. |
| 7C Deprecation schedule | Migrate internal `flowrender` callers; name removal release for `WarningWriter` in `docs/api-compatibility.md`. | No internal caller of a deprecated symbol; schedule published. | No `Deprecated:` markers remain under `internal/`. `WarningWriter`/`WithWarningWriter` carry `Deprecated:` comments naming v0.3.0 removal; table updated. |
| 7D Floor and citation refresh | `scripts/coverage-baseline.txt`; `docs/phase5-performance.md`; `docs/release.md` citation rule. | Floors within two points of matrix minimum; no unresolvable commit references. | Floors re-recorded from the four-cell matrix (Linux/macOS x Go 1.26.6/1.27.1); every floor equals its matrix minimum. Stale hash replaced by commit subject; citation rule added to `docs/release.md`. |

**Exit:** A reintroduced unused function or discarded error fails CI. Verified
by probe on 2026-09-08.

### Phase 8: Error-Handling Consistency (completed 2026-09-08)

**Goal:** One error policy across all packages.
**Covers:** N03, N04, N05, N11.

| Work item | Scope and dependencies | Completion gate | Result |
| --- | --- | --- | --- |
| 8A Limit marker | Internal marker interface/sentinel; facade mapping; per-limit tests. | `errors.Is` identifies every limit failure. | `internal/limit.ErrExceeded` is re-exported as `csspdf.ErrLimitExceeded`; all six concrete limit error families match it through wrapping. Diagnostic and operational-boundary classification use the sentinel rather than concrete type lists. |
| 8B Wrap audit | `pdfdom`, `pdfrender`, `i18n` error sites; classify leaf versus wrapped in PR description. | Causes preserved; `errors.Is(err, fs.ErrNotExist)` works through the facade for file sources. | Delegated parse, I/O, cancellation, template, layout, and rendering errors use `%w` or direct return; messages created solely from invalid local values remain leaf errors. A facade regression test proves `fs.ErrNotExist` survives `DiagnosticError` wrapping. |
| 8C Builder errors | `pdfdom` `Add`/`AddLine` and 33 call sites, or facade `recover` boundary. | Invalid child yields `DiagnosticError`; no panic reaches callers. | `PDFElementNode.Add` and `AddLine` return validation errors without mutation. Every parser call site propagates the error, checked test builders replace fluent panic-prone construction, and create-template parsing no longer discards nested errors. |
| 8D Span attribute strictness | `span_style.go` and tests. | Invalid numeric attributes fail in strict mode, warn in legacy mode. | `font-size` and `border-width` use the shared length parser and reject invalid, non-finite, or out-of-range values. Strict parsing returns contextual errors; legacy rendering warns and ignores only the invalid declaration. Unit-suffixed values are tested. |
| 8E Legacy path decision | Owner decision; both-modes tests at each `recoverableRenderError` site. | Removal release named or rationale recorded. | `AllowPartialRender` and `WithLegacyPartialRendering` are deprecated for removal in v0.3.0. Nested template/footer renderers return wrapped causes to one policy boundary; a strict/legacy matrix covers every recoverable renderer branch. |

**Exit:** Public documentation states the single error contract; a fuzz
target on HTML input finds no panics. Verified by the public render fuzz target
and a 67,301-execution local campaign on 2026-09-08; the bounded target runs in
the security workflow.

### Phase 9: Core Package Test Depth (completed 2026-09-08)

**Goal:** Unit-level confidence in parsing and layout.
**Covers:** N01, N10.

| Work item | Scope and dependencies | Completion gate | Result |
| --- | --- | --- | --- |
| 9A Option table test | `render_options_test.go`. | Every `With*` asserted; 100% of `render_options.go`. | Every public option is asserted, including defensive-copy behavior; all functions in `render_options.go` measure 100%. |
| 9B Templating characterization | `docflow_parser.go` exports, `page_css.go` edge cases. | `internal/templating` at or above 60%. | Selector application, prepared stylesheets, node helpers, CSS support diagnostics, page sizes, margins, and invalid declarations are characterized; coverage is 92.4%. |
| 9C PDFDOM characterization | `span_style.go`, `html_parser.go` branches, `elements.go` validation. | `internal/pdfdom` at or above 65%. | All element families, child validation, formatting, style inheritance, strict/legacy spans, and prepared parsing are covered; coverage is 76.2%. A nil-receiver panic in `SetAttribute` was fixed. |
| 9D Renderer characterization | Table layout, text continuation, use-template, flow margins. | `internal/pdfrender` at or above 72%. | Text/image helpers, table-cell parsing, flow bounds, page metadata, callbacks, and cancellation are characterized; coverage is 75.6%. |
| 9E Dumper tests and consolidation | Synthetic-PDF golden tests; one CLI entry point. | `internal/pdfdump` at or above 50%; `dump-pdf` duplication removed. | Dictionary, PNG predictor, xref, text-array, binary, bounded-I/O, and synthetic compressed-PDF paths are covered at 90.6%. `cmd/pdfdump` is the sole inspection CLI. |
| 9F Test file split | `render_test.go`, `sources_test.go` by concern. | No test file above 600 lines. | Renderer and source tests are split by concern; the largest test file is 597 lines. |

**Exit:** Overall coverage is 78.7%. Package floors were re-recorded after
Phase 10 from Go 1.26.6 and Go 1.27.1 profiles using the supported-matrix
minimum for each package; all exceed their required targets.

### Phase 10: Complexity Headroom and Backend Module

**Goal:** Keep the ratchet meaningful and the backend current.
**Covers:** N06, N07.

| Work item | Scope and dependencies | Completion gate |
| --- | --- | --- |
| 10A At-ceiling refactors | Four functions at complexity 15, one PR each, behind Phase 9 tests. | Complete: complexities are 4, 5, 9, and 6; focused characterization tests pass. |
| 10B Near-budget files | `text_engine.go`, `html_parser.go`, `elements.go`. | Complete: cohesive glyph registry, HTML value builder, and value formatting extractions leave the files at 545, 518, and 514 lines. |
| 10C Tighten ratchet | `MAX_CYCLO` 12 for changed scope; `MAX_FILE_LINES` 550. | Complete: defaults are 12 and 550; all-scope and worktree gates pass. |
| 10D Backend language version | `third_party/gofpdf/go.mod` directive and vestigial replace; `PATCHES.md`. | Complete: Go 1.22, no self-replace, semantic review recorded; nested vet and tests pass. |

**Exit:** Complete. No first-party function is above 12, no production file is
above 550 lines, and both modules declare a language version at or above 1.22.

## Copilot Work Item Protocol

The Phase 0-6 protocol applies unchanged. In summary: one behavioral
contract per PR; write the failing test first and confirm it fails for the
intended reason; implement the smallest change; run affected-package tests,
then `scripts/check-quality.sh` and `scripts/check-maintainability.sh`;
report exact commands and results; do not bundle dependency upgrades,
mechanical extraction, and behavior changes. The human owner approves any
compatibility decision before implementation starts.

## Maintainer Decisions

Phase 8 resolved the builder-error and legacy-rendering decisions. Phase 9
retained `cmd/pdfdump` as the sole inspection command and adopted the
proposed coverage targets. Phase 10 selected Go 1.22 for
`third_party/gofpdf`, the lowest version with per-iteration loop semantics,
while leaving the root module's newer minimum independent.
