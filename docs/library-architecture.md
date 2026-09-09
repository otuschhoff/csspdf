# Library Architecture

## Goal

Provide a reusable Go library that renders PDFs from declarative assets:
- HTML templates
- CSS
- flow JSON
- data JSON or native Go objects

## Layers

- Public engine: csspdf
- Rendering internals: internal/pdfrender, internal/pdfdom, internal/templating, internal/flowrender
- Reference profile and assets: examples/layered
- Repository-owned PDF backend: third_party/gofpdf

The root module has no sibling-checkout dependency. The backend source is kept
as a nested module because required fork APIs and UTF-8 fixes are not all
available from an externally published revision. See its `PATCHES.md` for
provenance and update requirements.

## Public API

Package: github.com/otuschhoff/csspdf

Key entry points:
- Render(outputPath, options...)
- RenderWithInput(RenderInput)
- RenderToFile(RenderInput, outputPath)
- RenderToWriter(RenderInput, io.Writer)
- RenderToBytes(RenderInput)

## Input Flexibility

csspdf supports:
- TextSource: bytes, string, file path, io/fs path
- JSONSource: Go object, JSON bytes/string, file path, io/fs path

This removes mandatory caller-side serialization logic.

## Determinism and Diagnostics

RenderInput supports:
- Now func() time.Time for deterministic function behavior
- Logger interface for non-fatal warnings
- FuncMapFactoryEx(FuncContext) for context-aware template functions

## Validation

Flow validation enforces:
- supported transformer names
- required section metadata
- payload path syntax
- runtime expression syntax

## Adapter Guidance

Profile-specific defaults and template function maps should live outside the core engine.

The example directories contain assets consumed by `cmd/gen-example`; they are
not Go packages or stable library APIs.
