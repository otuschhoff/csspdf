# Root Package Structure Phase 0 Discovery

Date: 2026-09-09
Status: Historical evidence (completed)
Baseline commit: `0964f76252f267812346a1b7e9c2dfdc3ed11cb7`

## Scope

This document closes Phase 0 of the root package structure initiative. It
records the current public API, assigns every root Go file to a durable owner or
split plan, identifies test coupling and rename-sensitive documentation, checks
the proposed dependency direction, and records a clean executable baseline.

Phase 0 changes documentation only. Production file renames and splits begin in
Phase 1 and Phase 2.

## Baseline Environment

The discovery and validation were performed with:

```text
Go:       go1.27.1
GOOS:     darwin
GOARCH:   arm64
GOWORK:   off
Module:   github.com/otuschhoff/csspdf
Revision: 0964f76252f267812346a1b7e9c2dfdc3ed11cb7
```

The root package contains:

- 22 production Go files;
- 16 same-package test files using `package csspdf`;
- one black-box test file using `package csspdf_test`; and
- 6,928 total lines across production and test Go files.

The largest root files at this baseline are:

| File | Lines | Observation |
| --- | ---: | --- |
| `sources_test.go` | 597 | At the test-file budget and spans source, base-directory, flow-default, CSS-layer, and HTML-layer behavior. |
| `render_test.go` | 571 | Spans core output, determinism, options, dimensions, i18n, policy, and layering. |
| `render_layout_test.go` | 396 | Spans CSS integration, output atomicity, i18n sources, and payload transformation. |
| `render.go` | 379 | Mixes public entry points, the public input model, emission, flow execution, and data helpers. |
| `render_resources_test.go` | 373 | Spans flow defaults, fonts, images, i18n, options, and macro errors. |
| `phase3_security_test.go` | 365 | Spans confinement, all render budgets, cancellation, and error policy. |
| `types.go` | 352 | Mixes public model declarations, layer composition, flow validation, and cloning helpers. |
| `sources.go` | 348 | Mixes public source models, decoding mechanics, layer inputs, and asset input resolution. |
| `template_funcs.go` | 320 | Contains public factories plus all private generic helper implementations. |

These sizes do not by themselves require new packages. They identify the first
files whose responsibilities should be separated within the root package.

## Public API Baseline

The public surface was recorded with:

```sh
GOWORK=off go doc -all .
```

With Go 1.27.1, the complete 486-line, 16,122-byte output has this SHA-256
fingerprint:

```text
97239ef35d5bf832731fc110334a9f02ab18e4ca8867f800ebe69034ad9859e2
```

The full output includes function and method signatures, public struct fields,
interface methods, documentation, and deprecation notices. Future API-neutral
phases must regenerate it with the recorded toolchain and compare the
fingerprint, in addition to reviewing the named inventory below.

Phase 1 and Phase 2 are API-neutral. The following symbol groups must remain
available from `github.com/otuschhoff/csspdf`.

### Constants and variables

- `DefaultLocale`
- `DefaultCurrencyCode`
- `DefaultPageFormat`
- `PageOrientationPortrait`
- `PageOrientationLandscape`
- `UnsupportedCSSPropertyCode`
- `ErrLimitExceeded`

### Public functions

- CSS support: `SupportedCSSProperties`, `AnalyzeCSSSupport`
- limits: `DefaultRenderLimits`
- migration: `LegacyCSSSourceAsLayer`,
  `MigrateAssetInputLegacyCSSToSingleLayer`
- rendering: `Render`, `RenderContext`, `RenderWithInput`,
  `RenderWithInputContext`, `RenderToWriter`, `RenderToWriterContext`,
  `RenderToBytes`, `RenderToBytesContext`, `RenderToFile`, and
  `RenderToFileContext`
- template functions: `DefaultTemplateFuncMap`,
  `DefaultTemplateFuncMapWithContext`, and `FormatLocalizedTime`
- options: `WithAssetBaseDir`, `WithConfinedResourceRoot`,
  `WithTrustedFileAccess`, `WithRenderLimits`, `WithAssets`, `WithAssetInput`,
  `WithSourceData`, `WithI18nSource`, `WithI18nTemplateMacros`,
  `WithLegacyPartialRendering`, `WithFontRegistrations`, `WithPageSize`,
  `WithPageFormat`, `WithPageOrientation`, `WithPageWidth`, `WithPageHeight`,
  `WithDefaultLocale`, `WithDefaultCurrencyCode`, `WithDefaultMargins`,
  `WithPageMarginsLeftRight`, `WithPageMarginsTopBottom`, `WithNow`,
  `WithFuncMapFactoryEx`, `WithFuncMapFactory`, `WithLogger`, and
  `WithWarningWriter`

### Public types

- assets and flow: `AssetInput`, `Assets`, `CSSLayer`, `CSSLayerInput`, `Flow`,
  `HTMLLayer`, `HTMLLayerInput`, `PayloadConfig`, and `Section`
- diagnostics and limits: `BudgetError`, `DiagnosticCode`, `DiagnosticError`,
  `LimitError`, and `RenderLimits`
- rendering: `FontRegistration`, `PageMargins`, `RenderInput`, and
  `RenderOption`
- resources and sources: `ConfinedFileResolver`, `JSONSource`,
  `ResourceResolver`, `TextSource`, and `TrustedFileResolver`
- templates and logging: `FuncContext`, `FuncMapFactory`,
  `FuncMapFactoryWithContext`, and `Logger`
- CSS support alias: `CSSSupportDiagnostic`

### Public methods

- `AssetInput.ResolveAssets`
- `AssetInput.ResolveWithBaseDir`
- `Assets.Validate`
- `BudgetError.Error` and `BudgetError.Is`
- `CSSLayerInput.Resolve`
- `ConfinedFileResolver.ReadFile` and `ConfinedFileResolver.ReadDir`
- `DiagnosticError.Error` and `DiagnosticError.Unwrap`
- `Flow.Validate`
- `HTMLLayerInput.Resolve`
- `JSONSource.IsSet` and `JSONSource.DecodeInto`
- `LimitError.Error` and `LimitError.Is`
- `Section.Validate`
- `TextSource.IsSet` and `TextSource.Resolve`
- `TrustedFileResolver.ReadFile` and `TrustedFileResolver.ReadDir`

Deprecation annotations on `AllowPartialRender`, `WarningWriter`,
`WithLegacyPartialRendering`, and `WithWarningWriter` are part of the public
documentation baseline and remain governed by
`docs/architecture/api-compatibility.md`.

## Production File Ownership

Every root production file has one proposed owner or an explicit split plan.
The target names are provisional until Phase 1 confirms that each current file
still has the recorded responsibility.

| Current file | Current responsibility | Phase 1 owner or Phase 2 split |
| --- | --- | --- |
| `asset_resolution.go` | Resolves `AssetInput` HTML, CSS, flow, and source data. | Assets; rename to `assets_resolve.go`. |
| `budget_writer.go` | Enforces the rendered output byte budget. | Render output; rename to `render_output_budget.go`. Keep in root because it returns root `BudgetError`. |
| `css_support.go` | Public CSS support constant, alias, and analysis functions. | Public API; rename to `api_css.go`. |
| `diagnostics.go` | Public diagnostic model plus private classification helpers. | Public API; rename to `api_diagnostics.go`; a later split is unnecessary unless private classification grows. |
| `doc.go` | Root package documentation. | Public API; keep `doc.go`. |
| `flow_defaults.go` | Infers default flow sections from template definitions. | Assets; rename to `assets_flow_defaults.go`. |
| `font_resources.go` | Discovers, normalizes, and deduplicates host font registrations. | Assets; rename to `assets_fonts.go`. |
| `limits.go` | Public render budgets, defaults, normalization, and `BudgetError`. | Public API; rename to `api_limits.go`. Keep normalization with the contract until a cycle-free boundary exists. |
| `migration_helpers.go` | Public compatibility helpers for CSS-layer migration. | Public API; rename to `api_migration.go`. |
| `output.go` | Public file render entry points and private atomic replacement mechanics. | Split in Phase 2/4: root `api_render_file.go` plus candidate `internal/fileout/atomic.go`. |
| `page_dimensions.go` | Normalizes and validates page size and orientation. | Render preparation; rename to `render_page_dimensions.go`. |
| `payload_transform.go` | Builds section payloads, resolves paths, and expands i18n macros. | Payload; keep name initially, then consider `payload_i18n.go` only if a cohesive split reduces the file. |
| `profile_templates.go` | Registers legacy profile-specific PDF template factories. | Compatibility; rename to `profile_legacy.go`. |
| `render.go` | Public render entry points and `RenderInput`, plus emission, flow execution, image paths, function-map construction, and JSON normalization. | Explicit split in Phase 2: `api_render.go`, `api_render_types.go`, `render_pipeline.go`, and narrowly named helper files based on final call ownership. |
| `render_options.go` | Public option API plus private input construction. | Public API; rename to `api_options.go`. |
| `render_policy.go` | Page-number callback setup and strict/legacy error policy. | Render orchestration; keep `render_policy.go`. |
| `render_preparation.go` | Builds assets, layout, source data, and final render artifact. | Render orchestration; rename to `render_prepare.go`; remain in root because it coordinates public root models. |
| `render_resources.go` | Resolves base assets, i18n, confined fonts, images, and resource type validation. | Explicit split in Phase 2: `assets_base.go`, `assets_i18n.go`, `assets_fonts_confined.go`, and `assets_images.go`; remain in root initially. |
| `resource_resolver.go` | Public resolver interface and implementations plus bounded regular-file reading. | Public API; rename to `api_resources.go`. Private reading remains colocated unless an extraction demonstrably simplifies it. |
| `sources.go` | Public text/JSON/layer/asset input models plus source decoding and resolution. | Explicit split in Phase 2: `api_sources.go`, `api_asset_inputs.go`, and private `assets_source_decode.go`, all in root. |
| `template_funcs.go` | Public template function factories and private date, locale, currency, aggregate, and conversion helpers. | Public API; rename to `api_template_funcs.go`; split private helper families only if navigation remains poor afterward. |
| `types.go` | Public shared models plus HTML/CSS composition, flow/payload validation, and clone helpers. | Explicit split in Phase 2: `api_types.go`, `api_template_types.go`, `assets_layers.go`, `assets_validation.go`, and `payload_validation.go`. |

## Test File Ownership and Coupling

Same-package tests deliberately exercise private root behavior. File renames do
not affect this access, but moving implementation into new packages would. The
test plan therefore distinguishes simple renames from behavioral splits.

| Current test file | Behavior owned | Plan |
| --- | --- | --- |
| `css_support_test.go` | Public CSS support contract. | Rename with `api_css.go` to `api_css_test.go`. |
| `diagnostics_test.go` | Diagnostic wrapping, provenance, and error classification. | Rename to `api_diagnostics_test.go`. |
| `external_consumer_test.go` | Black-box module build and render. | Keep unchanged as the canonical public-boundary test. |
| `font_emission_integration_test.go` | UTF-8 font emission and ToUnicode semantics. | Keep as a focused integration test. |
| `limit_errors_test.go` | Shared limit sentinel and operational classification. | Rename to `api_limits_test.go` or merge only with related limit tests. |
| `performance_test.go` | End-to-end render benchmarks. | Rename to `render_benchmark_test.go`; update performance documentation. |
| `phase3_security_test.go` | Resource confinement, budgets, cancellation, and strict error policy. | Split into `resource_security_test.go`, `render_budgets_test.go`, and `render_cancellation_test.go`. |
| `render_layout_test.go` | CSS integration, file output, i18n sources, and payload page data. | Split into `render_css_test.go`, `render_output_test.go`, `assets_i18n_test.go`, and `payload_transform_test.go`. |
| `render_options_test.go` | Option setters and default input construction. | Rename to `api_options_test.go`. |
| `render_resources_test.go` | Flow defaults, fonts, images, i18n, macros, and option integration. | Split into `assets_flow_defaults_test.go`, `assets_fonts_test.go`, `assets_images_test.go`, and `assets_i18n_test.go`; move option-only coverage to `api_options_test.go`. |
| `render_test.go` | Core output, determinism, options, page dimensions, i18n, error policy, and layers. | Split into `render_pipeline_test.go`, `render_determinism_test.go`, `render_page_dimensions_test.go`, `render_policy_test.go`, and domain-specific asset tests. |
| `resource_resolver_test.go` | Confined resolver safety, limits, and cancellation. | Rename to `api_resources_test.go`. |
| `security_fuzz_test.go` | JSON/flow validation and HTML render panic resistance. | Split only by fuzz domain if the file grows; keep fuzz targets discoverable. |
| `sources_migration_test.go` | HTML-layer composition and CSS migration helpers. | Split into `api_migration_test.go` and `assets_html_layers_test.go`. |
| `sources_test.go` | Source decoding, base defaults, flow defaults, and CSS/HTML layers. | Split into `api_sources_test.go`, `assets_base_test.go`, `assets_css_layers_test.go`, and `assets_html_layers_test.go`. |
| `template_funcs_test.go` | Public generic template helper behavior. | Rename to `api_template_funcs_test.go`. |
| `types_test.go` | Layer composition, validation, and clone behavior. | Split alongside `types.go` into `assets_validation_test.go`, `payload_validation_test.go`, and the relevant API model tests. |

### Direct private-function coverage

The following private production functions are referenced directly by root
tests. Their tests must move or be rewritten together if these functions cross
a package boundary:

| Domain | Private functions under direct test | Test files |
| --- | --- | --- |
| asset layers | `composeTemplateCSS`, `composeTemplateHTML`, `effectiveTemplateCSS`, `effectiveTemplateHTML`, `templateSourcesInRenderOrder` | `types_test.go`, `render_layout_test.go`, `sources_test.go`, `sources_migration_test.go` |
| fonts and images | `normalizeFontRegistration`, `resolveFontRegistrations`, `resolveImageSearchDirs` | `render_resources_test.go` |
| i18n and payload | `renderI18nTemplateNode`, `renderI18nTemplateNodeWithOptions`, `resolveI18nInput`, `transformGenericSection` | `render_resources_test.go`, `phase3_security_test.go`, `render_layout_test.go` |
| render pipeline | `buildArtifactContext`, `emitArtifactContext`, `pageNumberTemplateFlowElements`, `renderMainFlow` | `phase3_security_test.go` |
| options and dimensions | `buildRenderInput`, `resolvePageDimensions` | `render_options_test.go`, `render_resources_test.go`, `render_test.go` |
| policy and diagnostics | `diagnostic`, `diagnosticCode`, `isOperationalBoundaryError`, `normalizeRenderLimits` | diagnostic, limit, render-resource, render, and security tests |
| output | `writeFileAtomically` | `render_layout_test.go` |
| utility behavior | `cloneFlow`, `decodeJSON`, `formatResolvedCSSLayers` | `types_test.go`, `render_test.go`, `security_fuzz_test.go` |

The baseline contains 26 distinct private root functions with confirmed direct
test references. A lexical scan also matched the overloaded identifier
`resolve`, but no root test calls a private `.resolve` method directly, so that
ambiguous match is excluded. This coupling is not a defect: it records the
migration cost that a package extraction must account for.

## Dependency Direction Findings

The current internal dependency direction is clean:

```sh
rg '"github.com/otuschhoff/csspdf"' internal --glob '*.go'
```

The command returns no matches. No internal package imports the public root
package.

### Approved package-extraction candidate

`internal/fileout` is the only extraction approved for investigation by this
phase. The atomic replacement mechanism can be modeled using standard-library
types, an output path, and bytes. Root `RenderToFile` entry points can retain
render-domain error context and delegate the final write.

The extraction remains conditional on preserving focused failure injection for
temporary-file creation, permission preservation, writing, close, rename, and
cleanup behavior.

### Code that remains in root

The following clusters remain in root through Phase 2 because they consume the
public model and coordinate existing internal packages:

- asset input and base-directory resolution;
- render preparation and policy;
- page dimensions;
- resource, i18n, font, and image orchestration;
- payload transformation; and
- render limit normalization and diagnostic classification.

Creating `internal/assets`, `internal/prepare`, or `internal/payload` now would
require importing `csspdf`, moving public types behind aliases, duplicating
models, or introducing callback-heavy adapters. None is justified by the
current structure.

### Rejected generic packages

No `internal/helpers`, `internal/common`, or `internal/utils` package is
proposed. These names would hide ownership rather than establish a dependency
boundary.

## Rename-sensitive Documentation

Phase 1 must review these documents when files are renamed:

| Document | Current references | Required treatment |
| --- | --- | --- |
| `docs/maintenance/root-package-structure-initiative.md` | Proposed current and target filenames. | Update implementation status and any target name changed during execution. |
| `docs/maintenance/code-quality-review.md` | `output.go`, `sources_test.go`, `render_test.go`, `render_options.go`, `render_options_test.go`, `diagnostics.go`, `limits.go`, and `resource_resolver.go`. | Preserve historical evidence wording where it describes the review baseline; update links or present-tense claims where applicable. |
| `docs/maintenance/performance.md` | `performance_test.go`. | Update to `render_benchmark_test.go` when renamed. |

No root Go filename references were found in `README.md`, `SECURITY.md`, or the
quality and maintainability scripts at this baseline.

## Executable Baseline

All Phase 0 baseline checks passed on 2026-09-09.

### Full quality gate

```sh
GOWORK=off scripts/check-quality.sh
```

Result:

- module manifests tidy and all modules verified;
- formatting and build checks passed;
- `go vet`, staticcheck v0.8.1, errcheck v1.20.0, and error-wrap checks passed;
- all tests passed;
- every package coverage floor passed;
- root package coverage was 79.2% against a 78.6% floor; and
- govulncheck v1.7.0 found zero reachable vulnerabilities.

### Race suite

```sh
GOWORK=off go test -race ./... -count=1
```

Result: all packages passed with no reported races.

### Maintainability and architecture

```sh
GOWORK=off CHECK_SCOPE=all scripts/check-maintainability.sh
```

Result: complexity, file length, function length, and architecture boundary
checks passed.

### External consumer

```sh
GOWORK=off go test . \
  -run TestExternalConsumerBuildsAndRendersFromCleanDirectory \
  -count=1 -v
```

Result: the black-box consumer built and rendered successfully from an unrelated
temporary directory.

### Module stability

```sh
GOWORK=off go mod tidy -diff
(cd testdata/external-consumer && GOWORK=off go mod tidy -diff)
```

Result: both commands produced no diff.

## Phase 0 Decisions

### Decision A: organize the root before extracting packages

Phase 1 will use filename conventions and mechanical renames. Phase 2 will
split mixed-responsibility files while retaining package `csspdf`. This gives
the root a visible architecture without changing the dependency graph.

### Decision B: preserve the public API exactly

The `go doc -all .` inventory in this document is the comparison baseline.
Phase 1 and Phase 2 must not add, remove, rename, or change the documentation of
an exported symbol except to correct an independently reviewed documentation
defect.

### Decision C: do not create model aliases to enable directory moves

Public models remain declared in the root package. Internal aliases or mirrored
models are not warranted solely to move implementation files out of the root.

### Decision D: treat test coupling as part of each move

Every split must identify same-package tests that call the affected private
functions. Tests should be split by durable behavior at the same time, without
exporting production internals for test access.

### Decision E: investigate only atomic file output as an extraction

`internal/fileout` may be designed in Phase 4. No other new internal package is
approved by Phase 0 evidence.

## Exit Criteria Evaluation

| Criterion | Evidence | Status |
| --- | --- | --- |
| Every root Go file has one proposed owner or an explicit split plan. | The production and test ownership tables cover all 22 production files and all 17 test files reported by `go list`. | Complete |
| No proposed internal package requires importing the root package. | Current internal imports contain no root import. The only approved candidate, `internal/fileout`, can use standard-library inputs; root-coupled clusters remain in root. | Complete |
| The external-consumer fixture and full quality gate pass before changes. | Both checks passed at the recorded baseline; race, maintainability, and module stability checks also passed. | Complete |
| Exported root-package surface is recorded. | Constants, variables, functions, types, methods, and deprecation-sensitive fields/options are listed from `go doc -all .`. | Complete |
| Private-test coupling is identified. | Package styles and 23 directly referenced private functions are recorded by domain and test file. | Complete |
| Rename-sensitive documentation is identified. | Three affected documents are listed; README, SECURITY, and scripts have no current root-filename references. | Complete |

## Phase 0 Conclusion

Phase 0 is complete with no open discovery findings. Phase 1 may begin with
mechanical filename changes, using the ownership tables and public API inventory
in this document as its review contract.