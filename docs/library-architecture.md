# Library Architecture

## Goal

Provide a reusable Go library that renders PDFs from declarative assets:
- HTML templates
- CSS
- flow JSON
- data JSON or native Go objects

## Layers

- Public engine: docflowpdf
- Rendering internals: internal/pdfrender, internal/pdfdom, internal/template, internal/templateflow
- Example profile package: examples/invoice

## Public API

Package: github.com/otuschhoff/invoice-gen/docflowpdf

Key entry points:
- Render(RenderInput)
- RenderToFile(RenderInput, outputPath)
- RenderToWriter(RenderInput, io.Writer)
- RenderToBytes(RenderInput)

## Input Flexibility

docflowpdf supports:
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

For invoice profile in this repository:
- examples/invoice provides a reference profile package
