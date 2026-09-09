# Multi-layer CSS Implementation Roadmap

Status: Historical roadmap (completed)

## Objective

Enable ordered CSS layering so a reusable corporate identity style can be combined with document-specific styles, with deterministic override behavior.

## Why this matters

Current rendering accepts a single CSS text input and applies a simplified cascade to HTML nodes.
That works for one profile, but scales poorly when multiple documents must share a base design system.

Multi-layer CSS should let us:
- share common brand styles across many document templates
- keep document-level behavior local to each template
- support customer or environment specific overrides without copying base CSS
- maintain predictable output through explicit layer order

## Current state summary

Relevant current behavior:
- Assets has a single CSS string field.
- Rendering passes one CSS text blob into the style parser.
- CSS is mapped into node attributes via a curated property list, not a full browser engine.
- Inline attributes already have precedence and block later style writes for the same mapped attribute.

Implication:
- We can add multi-layer composition first, without changing the parser model.
- Full browser-grade cascade is not required for first delivery.

## Design principles from large CSS systems

Patterns used by larger ecosystems that are applicable here:
- strict layer ordering as a contract
- low specificity in reusable foundation layers
- targeted high-specificity selectors only in final override layers
- stable folder conventions by responsibility
- additive evolution: preserve backward compatibility while introducing layers

Patterns that are less applicable right now:
- full native CSS custom property resolution and advanced cascade semantics
- full CSS @layer implementation and browser-accurate specificity tie breaks

## Proposed model for this project

### Layer order contract

Layers are applied in array order from lowest to highest precedence.
Later layers may override earlier ones.

Recommended conceptual order:
1. Reset or base
2. Corporate tokens and primitives
3. Corporate components
4. Document template CSS
5. Customer or runtime override CSS

### API shape

Introduce layered CSS while preserving current API:
- keep existing Assets.CSS behavior unchanged
- add optional Assets.CSSLayers []CSSLayer

Proposed structures:
- CSSLayer
  - Name string
  - CSS string
  - Optional bool
- CSSLayerInput (for AssetInput resolution)
  - Name string
  - Source TextSource
  - Optional bool

Resolution behavior:
- if CSSLayers is empty, use existing CSS field exactly as today
- if CSSLayers is set, resolve each layer source to text and concatenate in order
- optionally append legacy CSS as final layer during migration if configured

## Repository organization proposal

Recommended style directories:
- styles/corporate/base.css
- styles/corporate/components.css
- styles/documents/invoice.css
- styles/overrides/customer-acme.css

Recommended usage contract:
- corporate layers never depend on document selectors
- document layer can override corporate defaults
- override layer is final and minimal

## Implementation roadmap

## Phase 0 - Discovery and constraints (short) - Completed 2026-08-02

Status:
- Completed. See phase deliverable note in [multi-layer-css-phase0-discovery.md](multi-layer-css-phase0-discovery.md).

Phase outputs delivered:
- parser property mapping limits are documented
- CSS ingress call paths are documented
- mixed-input first-release behavior is decided and locked

Deliverables:
- confirm property mapping limits in parser and document them: done
- identify all call paths that inject CSS into rendering: done
- decide whether to support mixed input (CSS + CSSLayers) in first release: done

Locked Phase 0 decisions:
- first release supports mixed input by composing explicit CSSLayers first and appending legacy CSS as implicit final layer
- when both modes are used, emit one transition warning through logger
- Phase 1 keeps existing parser and mapping semantics; only CSS composition input changes

Acceptance criteria:
- short design note added to docs: done
- team agreement on ordering contract and compatibility behavior: done

## Phase 1 - Data model and asset resolution - Completed 2026-08-02

Status:
- Completed in code and tests.

Phase outputs delivered:
- added resolved layer model: Assets.CSSLayers []CSSLayer
- added source-layer model: AssetInput.CSSLayers []CSSLayerInput
- added ordered CSS composition helper that appends legacy Assets.CSS as implicit final layer
- preserved legacy behavior when no layers are provided
- added optional/required layer resolution behavior

Tasks:
- add CSSLayer model and CSSLayers field in Assets: done
- extend AssetInput to resolve ordered CSS layer sources: done
- implement helper that builds effective CSS text from layers: done
- preserve current single CSS behavior when no layers are provided: done

Acceptance criteria:
- all current tests still pass without changes to existing callers: done
- new unit tests verify order-sensitive concatenation: done
- missing optional layers are ignored, required layers fail clearly: done

Verification snapshot:
- go test . ./internal/pdfrender ./internal/pdfdom: pass

## Phase 2 - Render pipeline integration - Completed 2026-08-02

Status:
- Completed in code and tests.

Phase outputs delivered:
- render path now builds one effective CSS string from layers plus legacy CSS composition before downstream parsing/rendering
- both main flow and page-number flow consume the same resolved CSS value
- logger diagnostics now include resolved CSS layer order (and mixed-mode notice when legacy CSS is also provided)

Tasks:
- switch render paths to use effective CSS text builder: done
- ensure both main flow and page number flow use identical resolved CSS: done
- add diagnostics for resolved layer list when logger is enabled: done

Acceptance criteria:
- rendering output remains unchanged for legacy single CSS input: done
- layered input produces deterministic overrides by order: done

Verification snapshot:
- go test . ./internal/pdfrender ./internal/pdfdom: pass

## Phase 3 - Testing strategy - Completed 2026-08-02

Status:
- Completed in code and tests.

Phase outputs delivered:
- strict order behavior with conflicting declarations is covered
- explicit fallback to legacy CSS field is covered
- mixed source types for layer input (inline text, file path, fs path) are covered
- optional vs required missing layer behavior is covered
- parser mapping behavior under layered overrides is covered for mapped properties

Add tests for:
- strict order behavior with conflicting declarations: done
- fallback to legacy CSS field: done
- mixed source types (inline text, file path, fs path): done
- optional layer missing vs required layer missing: done
- parser mapping behavior with layered overrides for mapped properties: done

Acceptance criteria:
- tests cover positive and negative paths: done
- no flaky order-dependent behavior: done

Verification snapshot:
- go test . ./internal/pdfrender ./internal/pdfdom: pass

## Phase 4 - Documentation and examples - Completed 2026-08-02

Status:
- Completed in documentation and runnable example assets.

Phase outputs delivered:
- added concept document for layer organization and operational rules
- added end-to-end layered example profile with corporate/document/override CSS layers
- updated README with layered CSS usage snippet and runnable command
- documented parser/cascade limitations explicitly in docs and README

Tasks:
- add a concept doc for layer organization rules: done
- add an example profile using corporate + document + override layers: done
- update README usage snippets for layered CSS: done

Acceptance criteria:
- user can copy one example and run it end-to-end: done
- docs explicitly state parser and cascade limitations: done

Verification snapshot:
- `go run ./cmd/gen-example layered -o output/layered.pdf`: pass
- `go test ./cmd/gen-example ./csspdf ./internal/pdfrender ./internal/pdfdom`: pass

## Phase 5 - Migration and rollout - Completed 2026-08-02

Status:
- Completed with migration helpers, rollout scripts, and documented migration/verification flow.

Phase outputs delivered:
- backward compatibility remains default: legacy single CSS inputs still render unchanged
- migration helper APIs added:
  - LegacyCSSSourceAsLayer
  - MigrateAssetInputLegacyCSSToSingleLayer
- rollout scripts added:
  - scripts/check-layered-rollout.sh
  - scripts/measure-layered-duplication.sh
- README updated with migration helper usage and rollout command

Migration strategy:
- release with backward compatible defaults: done
- recommend new projects adopt CSSLayers: done
- provide migration helper that converts one CSS file path into one-layer config: done

Rollout checks:
- run focused render tests for representative templates: done
- compare sample PDFs before and after enabling layers: done
- measure whether duplicated CSS decreases in example profiles: done

Verification snapshot:
- `go test . ./internal/pdfrender ./internal/pdfdom`: pass
- `./scripts/check-layered-rollout.sh`: pass
- generated outputs:
  - `output/phase5/invoice-legacy.pdf`
  - `output/phase5/layered.pdf`

## Risks and mitigations

Risk: users expect full browser cascade behavior.
Mitigation: document the supported property mapping and precedence model clearly.

Risk: layer ordering mistakes lead to silent style drift.
Mitigation: add optional runtime warnings for duplicate selectors across adjacent layers.

Risk: mixed legacy and layered modes become confusing.
Mitigation: define one clear precedence rule and keep it stable across versions.

## Suggested acceptance criteria for feature completion

Feature is done when:
- ordered multi-layer CSS is supported in API and asset loader
- legacy single CSS usage remains fully compatible
- tests cover ordering, fallback, and missing layer behavior
- at least one example demonstrates corporate + document override layering
- documentation provides clear usage and limitations

## Estimated delivery slices

A practical sequence for incremental delivery:
1. PR 1: data model + resolver + unit tests
2. PR 2: render integration + compatibility tests
3. PR 3: examples + documentation + migration notes

This keeps review scope manageable and lowers regression risk.
