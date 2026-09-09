# Multi-layer CSS Phase 0 Discovery and Decisions

Date: 2026-08-02
Status: Historical evidence (completed)

## Scope

Phase 0 goals from the roadmap:
- confirm property mapping limits in parser and document them
- identify all call paths that inject CSS into rendering
- decide mixed input behavior (CSS plus CSSLayers) for first release

## Findings: current CSS model and limits

## 1) CSS processing model

Current pipeline applies one CSS text blob to parsed HTML before conversion to PDFDOM nodes.

Mechanics:
- ParseStyledFragment parses HTML and then calls ApplyStylesheet.
- ApplyStylesheet parses CSS rules and queries matching nodes for each selector.
- Each declaration is converted into an HTML attribute through CSSDeclarationToAttr.
- Unknown CSS properties are ignored (no mapping, no error).

## 2) Supported mapped CSS properties

Mapped properties currently include:
- text-align
- position, top, right, bottom, left
- width, height
- fill
- border, border-width, border-style, border-color
- padding, padding-top, padding-right, padding-bottom, padding-left
- font-family, font-size, color, font-style, font-weight (bold/normal variants)
- background-color
- margin-top, margin-bottom
- break-before, break-after
- white-space
- table-layout

Properties outside this list are ignored.

## 3) Precedence and cascade characteristics

Current precedence behavior:
- inline attributes on elements win over stylesheet declarations for the same mapped attribute
- for non-inline attributes, last applied declaration wins due to SetOrReplaceAttr overwrite
- rule application order follows stylesheet order as parsed

Known limitations relative to browser CSS engines:
- no full CSS cascade implementation
- no explicit !important handling
- no native CSS variable resolution model
- no full @layer semantics

These limits are acceptable for Phase 1 layered composition because layer composition can happen before parsing while keeping deterministic ordering.

## Findings: CSS ingress call paths

Primary runtime path for document rendering:
- csspdf build step reads page settings from assets.CSS via ParseCSSPageSettings
- main flow rendering passes assets.CSS to BuildFlowElementsWithFuncs
- page number rendering passes assets.CSS to BuildFlowElementsWithFuncs
- flowrender passes cssStyle to ParseHTMLDocFlow
- ParseHTMLDocFlow uses ParseStyledFragment and ApplyStylesheet

Secondary parser entry points that accept cssStyle:
- ParseHTMLTableElem
- ParseHTMLSectionElem
- ParseHTMLIntroElem

Implication:
- all rendering paths should consume one shared "effective CSS" builder output once Phase 1 starts
- page settings parser and flow parser must use the same composed CSS text

## Phase 0 decisions (locked)

## Decision A: first release will support mixed legacy and layered input

When both are provided:
- explicit CSSLayers are applied first in declared order
- legacy Assets.CSS is appended as an implicit final layer
- resulting precedence is deterministic and backward friendly

Rationale:
- enables incremental migration
- prevents surprising behavior changes for existing templates
- keeps override intent intuitive: legacy CSS remains strongest while migrating

## Decision B: add transition warning when both modes are used

If both CSSLayers and legacy CSS are present:
- emit one warning through existing logger path
- message states that legacy CSS is treated as final layer

Rationale:
- preserves compatibility while making behavior visible
- encourages full migration to layers over time

## Decision C: Phase 1 remains parser-model preserving

Phase 1 will not change selector engine or mapping semantics.
Only composition and resolution inputs change.

Rationale:
- reduces risk and keeps scope focused on layering contract
- allows later work on advanced CSS semantics as separate effort

## Exit criteria check

Phase 0 deliverables status:
- property mapping limits documented: done
- CSS ingress call paths identified: done
- mixed-input first-release decision recorded: done

Roadmap update required by this phase:
- mark Phase 0 complete and reference this document
