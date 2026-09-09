# Root Package Structure Initiative

Date: 2026-09-09
Status: Completed (Phases 0-6)

## Goal

Make the repository easier to navigate and maintain without changing the
supported import path or public API.

The root package remains:

```text
github.com/otuschhoff/csspdf
```

The initiative will organize root-package files by responsibility, split files
that currently mix unrelated concerns, extract only implementation mechanics
with clean dependency boundaries, and separate current architecture guidance
from historical project records.

Success means that a maintainer can quickly answer:

- which files define the supported public API;
- where render orchestration is implemented;
- where assets, resources, payloads, and output are handled;
- which packages own parsing, layout, and PDF rendering behavior; and
- which documents describe current contracts rather than historical work.

## Motivation

The internal packages are already organized around recognizable domains:

- `internal/templating`
- `internal/flowrender`
- `internal/pdfdom`
- `internal/pdfrender`
- `internal/format`
- `internal/i18n`
- `internal/pdfdump`

The largest structural pressure is at the module root. The root contains the
public `csspdf` package and currently has many Go files covering several kinds
of responsibility:

- public types and entry points;
- render options and compatibility helpers;
- input and resource resolution;
- render preparation and policy;
- output handling and operational limits;
- template payload transformation; and
- broad integration, security, migration, and performance tests.

All of these files appear together in directory listings because Go requires
files in one package to share a directory. Their current names do not always
make the public boundary or domain grouping obvious.

## Constraints

### Preserve the canonical package

Public callers must continue to use `github.com/otuschhoff/csspdf`. Existing
exported types, functions, methods, and option behavior must remain available
unless a separate compatibility decision explicitly changes them.

### Respect Go package dependencies

Private root implementations commonly use public root types such as:

- `RenderInput`
- `Assets` and `AssetInput`
- `Flow`, `Section`, and `PayloadConfig`
- `RenderLimits`
- `ResourceResolver`
- `DiagnosticError`

Moving such implementations directly into an `internal` package would not be a
mechanical file move. If the root package imports that internal package and the
internal package imports the root package for these types, Go reports an import
cycle.

This initiative therefore does not treat additional directories as an end in
themselves. A package extraction is justified only when it creates a clear,
one-way dependency and does not require duplicate models, callback-heavy APIs,
or conversion layers with little behavioral value.

### Preserve ownership boundaries

The dependency direction in `docs/architecture/boundaries.md` remains in force.
In particular, internal packages must not import the public root package, and
the existing parser, DOM, layout, and renderer responsibilities should not be
collapsed into a generic utility package.

### Keep changes reviewable

File moves, behavior changes, dependency upgrades, and public API changes must
not be combined into one migration step. Each phase should be independently
reviewable and leave the repository passing its normal quality gates.

## Current Root Organization

The first stage uses filename conventions to create visible groups while
keeping one Go package and one public import path.

```text
csspdf/
├── doc.go
│
├── api_asset_inputs.go
├── api_css.go
├── api_diagnostics.go
├── api_limits.go
├── api_migration.go
├── api_options.go
├── api_render.go
├── api_render_file.go
├── api_render_types.go
├── api_resources.go
├── api_sources.go
├── api_template_funcs.go
├── api_template_types.go
├── api_types.go
│
├── assets_base.go
├── assets_flow_defaults.go
├── assets_fonts.go
├── assets_fonts_confined.go
├── assets_fonts_layout.go
├── assets_i18n.go
├── assets_images.go
├── assets_layers.go
├── assets_render_order.go
├── assets_resolve.go
├── assets_source_decode.go
├── assets_validation.go
│
├── payload_clone.go
├── payload_json.go
├── payload_transform.go
├── payload_validation.go
│
├── render_context.go
├── render_errors.go
├── render_output_budget.go
├── render_page_dimensions.go
├── render_pipeline.go
├── render_policy.go
├── render_prepare.go
│
├── profile_legacy.go
├── template_aggregate.go
├── template_currency.go
├── template_date.go
│
└── internal/
  └── fileout/
    └── atomic.go
```

The exact filenames may change during implementation when a more cohesive
boundary becomes visible. The durable convention is:

- `api_`: supported public declarations and thin public entry points;
- `render_`: orchestration and rendering policy owned by the facade;
- `assets_`: asset composition, discovery, and resolution;
- `payload_`: payload transformation and validation;
- `profile_legacy`: compatibility behavior scheduled for eventual removal; and
- unprefixed domain names only where the responsibility is already unambiguous.

Tests should use the same domain vocabulary, for example
`assets_resolver_test.go`, `render_pipeline_test.go`, and
`resource_security_test.go`.

## Implications

### Public API

The initial phases are API-neutral. Renaming or splitting files does not change
Go symbols, package names, import paths, generated documentation, or consumer
code.

Public declarations should remain in the root package, including:

- render entry points and options;
- `RenderInput` and related configuration types;
- source abstractions;
- diagnostics and limits;
- resource resolver interfaces and implementations;
- supported template function factories; and
- migration helpers retained by the compatibility policy.

The external-consumer fixture remains the executable check that these contracts
are still usable from another module.

### Internal package design

Most render preparation and asset resolution should initially remain in the
root package. Their coupling to the public configuration model is legitimate:
the root facade owns normalization and orchestration.

Implementation may move under `internal/` when all of the following are true:

1. The extracted code has a cohesive responsibility.
2. Its inputs and outputs do not require importing `csspdf`.
3. The root-to-internal dependency remains one-way.
4. The extraction removes meaningful complexity or duplication.
5. Focused tests can validate the new boundary independently.

Atomic file replacement is a likely candidate because its mechanics can accept
a path and bytes without depending on render-domain types. Render preparation
is not currently a good candidate because it consumes most of the public input
model and coordinates several internal packages.

### Test layout

Tests should remain beside the package whose behavior they verify. Root-package
tests are appropriate for public behavior and facade orchestration; tests should
not be moved to an artificial test directory merely to shorten the root listing.

Large or phase-named test files should instead be split by durable behavior.
For example:

- render success, error policy, and layout integration;
- text source, JSON source, and layered source resolution;
- resource confinement and resource budgets;
- compatibility migrations; and
- benchmarks and fuzz targets.

Historical phase names should remain in evidence documents, not become the
permanent ownership model for tests.

### Documentation layout

Current architecture, operating guidance, maintenance procedures, historical
evidence, concepts, and release notes are grouped by ownership under `docs/`:

```text
docs/
├── README.md
├── architecture/
├── guides/
├── maintenance/
├── history/
├── concepts/
└── releases/
```

Published release-note paths remain stable. `scripts/check-doc-links.sh`
validates local Markdown targets, repository-root documentation references,
and complete index coverage as part of the quality gate.

### Generated artifacts

Generated artifacts remain outside the source structure and version control.
Repository-local binaries, rendered examples, rollout output, and persistent
benchmark profiles use the ignored `.build/` tree. Test-owned files remain in
`t.TempDir()`, and the coverage gate retains its auto-cleaned operating-system
temporary file. Legacy `bin/` and `output/` trees remain ignored so existing
local artifacts do not become visible or require destructive migration.

## Implementation Steps

### Phase 0: Inventory and lock the contract - Completed 2026-09-09

Status:

- Completed. See
  `docs/history/root-package-structure-phase0-discovery.md` for the API inventory,
  per-file ownership map, test-coupling analysis, dependency decisions, and
  executable baseline.

Tasks:

- record the exported root-package surface with `go doc`;
- map root files to public API, orchestration, assets, payloads, output, and
  compatibility responsibilities;
- identify tests that rely on package-private helpers;
- identify documentation links affected by proposed moves; and
- confirm a clean quality baseline before structural edits.

Acceptance criteria:

- every root Go file has one proposed owner or an explicit split plan: done;
- no proposed internal package requires importing the root package: done; and
- the external-consumer fixture and full quality gate pass before changes: done.

### Phase 1: Establish root filename conventions - Completed 2026-09-09

Status:

- Completed. Cohesive production and test files now use the `api_`, `assets_`,
  `render_`, and compatibility naming conventions. Mixed-responsibility files
  remain under their existing names for the Phase 2 split, avoiding temporary
  names and unnecessary rename churn.

Rename files without moving symbols between packages. Likely changes include:

- `render_options.go` to `api_options.go`;
- `sources.go` to `api_sources.go`;
- `resource_resolver.go` to `api_resources.go`;
- `diagnostics.go` to `api_diagnostics.go`;
- `limits.go` to `api_limits.go`;
- `migration_helpers.go` to `api_migration.go`;
- `asset_resolution.go` to `assets_resolve.go`;
- `font_resources.go` to `assets_fonts.go`;
- `flow_defaults.go` to `assets_flow_defaults.go`; and
- `profile_templates.go` to `profile_legacy.go`.

Names should be confirmed against actual file contents before each rename. If a
file spans multiple groups, split it instead of assigning a misleading prefix.

Additional confirmed renames from the Phase 0 ownership map:

- `css_support.go` to `api_css.go`;
- `budget_writer.go` to `render_output_budget.go`;
- `page_dimensions.go` to `render_page_dimensions.go`;
- `render_preparation.go` to `render_prepare.go`; and
- the seven focused test files corresponding to CSS, diagnostics, limits,
  options, resources, template functions, and benchmarks.

Deferred to Phase 2 because they span multiple target groups:

- `render.go`;
- `types.go`;
- `sources.go`;
- `output.go`;
- `render_resources.go`; and
- broad test files that must split by behavior rather than receive a misleading
  single-domain name.

Acceptance criteria:

- no symbol or package API changes: done;
- `go doc github.com/otuschhoff/csspdf` exposes the same supported surface:
  done, with an identical Phase 0 SHA-256 fingerprint;
- focused package tests and the external-consumer test pass: done; and
- history records moves as renames where practical: done.

### Phase 2: Split mixed-responsibility files - Completed 2026-09-09

Status:

- Completed. `types.go`, `sources.go`, `render.go`, `render_resources.go`, and
  `output.go` were replaced by cohesive root files without changing package
  boundaries. Private template helper families were also separated from the
  public template API after the phase audit found that file remained mixed.

Start with files that combine public models and private implementation. The
first candidate is `types.go`, which currently includes public structures,
template context types, asset composition, and payload validation.

Proposed split:

- `api_types.go`: public render, asset, flow, and layer models;
- `api_template_types.go`: logger and template function factory contracts;
- `assets_layers.go`: effective HTML and CSS composition;
- `assets_validation.go`: resolved asset validation; and
- `payload_validation.go`: flow, section, and payload path validation.

Acceptance criteria:

- behavior and exported declarations are unchanged: done, with all tests green
  and an identical API fingerprint;
- validation tests remain focused and readable: done for production ownership,
  with physical test-file splits assigned to Phase 3;
- no new package dependency is introduced: done; and
- all files remain within maintainability budgets: done, with no production
  root file above 300 lines.

### Phase 3: Reorganize root tests by behavior - Completed 2026-09-09

Status:

- Completed. Broad and phase-named root tests now use durable API, asset,
  payload, render, integration, security, fuzz, and benchmark ownership names.
  Mixed type tests were split by behavior, JSON source and page-dimension tests
  moved beside their owning APIs, and HTML-layer tests have a focused suite.

Split broad test files and replace temporary project-phase names with durable
domain names. Keep tests in package `csspdf` when they intentionally exercise
private orchestration; use package `csspdf_test` only for genuine black-box API
tests.

Acceptance criteria:

- no reduction in package coverage floors: done, with root statement coverage
  at 79.2% and all repository package tests passing;
- fuzz targets and benchmarks remain discoverable by standard Go tooling:
  done, with two fuzz targets and `BenchmarkRenderWorkloads` listed by
  `go test`;
- shared test helpers have clear ownership: done, with render fixtures in the
  render pipeline suite and layered-template helpers in the migration suite;
  and
- no production API is exported solely to support tests: done, with an
  unchanged public API fingerprint.

### Phase 4: Extract independent mechanics - Completed 2026-09-09

Status:

- Completed. Atomic replacement now belongs to the standard-library-only
  `internal/fileout` package. The root file-render facade validates its public
  input, renders bytes, and delegates the final write without exposing internal
  operations or adding public API. No additional extraction qualified: the
  remaining root orchestration consumes public render models and would require
  shared models, adapters, or a new design decision.

Evaluate small implementation boundaries after the root cleanup. The first
candidate is atomic file output:

```text
internal/fileout/
└── atomic.go
```

The root `RenderToFile` functions remain public and delegate only the final file
replacement operation. Error context visible to callers must remain compatible.

Other extractions require a separate design note if they need shared models,
new interfaces, or substantial parameter translation.

Acceptance criteria:

- the extracted package has no dependency on the root package: done, with only
  standard-library imports, a one-way root-to-internal dependency, and an
  explicit maintainability check preventing csspdf imports;
- tests cover permission preservation, cleanup, close failures, and replacement
  failures: done, including create, stat, chmod, write, close, rename, missing
  destination, temporary-name, and wrapped-cause checks, with a 100% enforced
  statement-coverage floor;
- errors retain useful operation and path context: done, with the existing
  caller-visible error messages and `%w` causes preserved; and
- root orchestration becomes smaller rather than merely redistributed: done,
  with `api_render_file.go` reduced to validation, rendering, and one atomic
  write delegation.

### Phase 5: Reorganize documentation - Completed 2026-09-09

Status:

- Completed. Current contracts, user guidance, maintenance procedures, and
  historical evidence now have separate ownership directories. `docs/README.md`
  indexes every document, while published release-note paths remain unchanged.
  The semantic audit also removed obsolete nested-backend steps from CI and
  corrected current troubleshooting guidance.

Separate current contracts from historical evidence. Move documents in one
reviewable change, update all repository links, and preserve release-note URLs
when external references make that necessary.

Acceptance criteria:

- all relative Markdown links resolve: done, enforced by
  `scripts/check-doc-links.sh` in the shared quality gate;
- README links point to current guidance: done, with the repository README
  linking the documentation index and current architecture, guides, and
  maintenance paths;
- historical records are clearly labeled as historical: done, through the
  history index description and explicit status labels on archived discovery,
  roadmap, baseline, and prior-release records; and
- architecture and release procedures do not contradict the current tree:
  done, after an audit of current docs and removal of stale nested-backend CI
  cache and race-test steps.

### Phase 6: Evaluate generated-artifact consolidation - Completed 2026-09-09

Decide whether `bin/`, `output/`, coverage profiles, and benchmark artifacts
should move under `.build/`. This phase is optional and should proceed only if
the resulting command and CI conventions are simpler.

Decision:

- repository-local CLI binaries use `.build/bin/`;
- example and rollout PDFs use `.build/output/`;
- persistent benchmark artifacts use `.build/profiles/`;
- temporary coverage profiles remain under `${TMPDIR:-/tmp}` and are removed
  automatically unless `COVERAGE_PROFILE` explicitly preserves one;
- test artifacts remain in `t.TempDir()`; and
- existing `bin/` and `output/` directories are not moved or deleted and stay
  ignored as legacy local artifact locations.

This partial consolidation is simpler than forcing every generated file into
one directory: persistent repository-local artifacts have one visible home,
while self-cleaning temporary files keep their existing lifecycle and do not
leave repository clutter.

Acceptance criteria:

- generated artifacts remain ignored: done, with `/.build/`, `/bin/`, and
  `/output/` rules covering the current and legacy local locations;
- examples and release checks use the same paths locally and in CI: done,
  because command defaults, documentation, and `check-layered-rollout.sh` use
  `.build/output/`, while the benchmark runner defaults to
  `.build/profiles/phase5`; and
- no source, fixture, or provenance-controlled asset moves into the generated
  directory: done; all tracked example inputs remain under `examples/`, and
  `git ls-files .build bin output` is empty.

## Validation Strategy

Run validation after each phase rather than waiting for the complete initiative.
At minimum:

```sh
go test ./... -count=1
go test -race ./... -count=1
CHECK_SCOPE=all scripts/check-maintainability.sh
scripts/check-quality.sh
```

Structural phases must additionally verify:

```sh
go mod tidy -diff
(cd testdata/external-consumer && go mod tidy -diff)
git diff --check
```

For API-neutral phases, compare the exported package documentation or an
equivalent API inventory before and after the change. The external-consumer test
must build and render without relying on a workspace or sibling checkout.

## Non-goals

This initiative does not:

- change the canonical package path;
- introduce a second public package hierarchy;
- redesign rendering behavior or CSS semantics;
- remove compatibility APIs ahead of their documented schedule;
- create generic `helpers`, `common`, or `utils` packages;
- move tightly coupled code merely to reduce the number of root files;
- combine dependency upgrades with structural changes; or
- reorganize the gofpdf fork, which has its own repository and quality gates.

## Decision Rules

When choosing between a root-file split and a new internal package, prefer the
root-file split unless the package boundary has a clear independent model and a
one-way dependency.

When choosing between fewer files and more cohesive files, prefer cohesion. The
goal is not a minimum file count; it is predictable ownership and navigation.

When a proposed move requires new public symbols, duplicated models, or several
callbacks solely to avoid an import cycle, stop and write a focused design note
before implementation.

## Completion Criteria

The initiative is complete when:

- root filenames visibly distinguish public API from implementation domains;
- mixed-responsibility files have been split along stable boundaries;
- broad tests are organized by behavior rather than historical phase;
- any new internal package has a justified, cycle-free dependency boundary;
- current and historical documentation are clearly separated;
- generated artifacts have one documented policy;
- the supported public API and import path are unchanged; and
- all quality, race, maintainability, coverage, and external-consumer gates
  pass on the final structure.