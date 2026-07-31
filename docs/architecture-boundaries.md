# Architecture Boundaries

This document defines package responsibilities and dependency direction checks
that are enforced by `scripts/check-maintainability.sh`.

## Package Responsibilities

- `internal/domain`
- Pure business/domain data structures only.
- No rendering, orchestration, or template parsing logic.

- `internal/templating`
- Template execution/parsing and CSS/page/docflow parsing only.
- No business orchestration or PDF rendering dependencies.

- `internal/flowrender`
- Adapter layer: named template + CSS => PDFDOM element flow.
- No business orchestration imports.

- `internal/invoice`
- Use-case orchestration and data preparation.
- Chooses template sections and drives rendering flow.

- `internal/pdfrender`
- Low-level PDF/layout rendering primitives.

## Enforced Import Rules

- `internal/domain` must not import:
- `internal/invoice`
- `internal/pdfrender`
- `internal/flowrender`
- `internal/app`
- `internal/templating`

- `internal/templating` must not import:
- `internal/invoice`
- `internal/pdfrender`
- `internal/app`

- `internal/flowrender` must not import:
- `internal/invoice`
- `internal/app`

## Maintainability Budgets

Default thresholds (can be overridden by env vars):

- Cyclomatic complexity (`MAX_CYCLO`): `15`
- Function length in non-test Go files (`MAX_FUNC_LINES`): `80`
- Go file length (`MAX_FILE_LINES`): `600`

## Local Usage

```sh
chmod +x scripts/check-maintainability.sh
scripts/check-maintainability.sh
```

If `gocyclo` is missing:

```sh
go install github.com/fzipp/gocyclo/cmd/gocyclo@latest
```
