# Architecture Boundaries

This document defines package responsibilities and dependency direction checks
that are enforced by `scripts/check-maintainability.sh`.

## Package Responsibilities

- `docflowpdf`
- Public API, input normalization, and render orchestration.
- May compose internal packages; internal packages must not import it.

- `internal/templating`
- Template execution/parsing and CSS/page/docflow parsing only.
- No business orchestration or PDF rendering dependencies.

- `internal/flowrender`
- Adapter layer: named template + CSS => PDFDOM element flow.
- No business orchestration imports.

- `internal/pdfdom`
- PDF-oriented document node model and HTML-to-node conversion.
- No public API or orchestration dependencies.

- `internal/pdfrender`
- Low-level PDF/layout rendering primitives.

- `internal/format` and `internal/i18n`
- Generic formatting and translation support.
- No dependency on rendering orchestration.

- `internal/pdfdump`
- Diagnostic PDF inspection used by repository commands.

- `third_party/gofpdf`
- Repository-owned external backend source with a separately documented patch
	set and nested module tests.
- Excluded from csspdf maintainability budgets; changes require both nested and
	root quality gates.

## Enforced Import Rules

- `internal/templating` must not import:
- `internal/pdfrender`
- `docflowpdf`

- `internal/flowrender` must not import:
- `docflowpdf`

## Maintainability Budgets

Default thresholds (can be overridden by env vars):

- Cyclomatic complexity (`MAX_CYCLO`): `15`
- Function length in non-test Go files (`MAX_FUNC_LINES`): `80`
- Go file length (`MAX_FILE_LINES`): `600`

## Local Usage

```sh
go install github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0
scripts/check-maintainability.sh
```

The default `worktree` scope checks modified and untracked project Go files.
CI uses `CHECK_SCOPE=changed` with an explicit base revision. Use
`CHECK_SCOPE=all` to inspect all owned Go files; existing debt above the ratchet
will be reported.

Run the complete build/test/security gate separately:

```sh
scripts/check-quality.sh
```
