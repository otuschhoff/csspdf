# Architecture Boundaries

This document defines package responsibilities and dependency direction checks
that are enforced by `scripts/check-maintainability.sh`.

## Package Responsibilities

- `csspdf`
- Public API, input normalization, and render orchestration.
- May compose internal packages; internal packages must not import it.
- Owns public compatibility adapters, including legacy profile-template names;
	generic internal layout remains profile-neutral.

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

- `github.com/otuschhoff/gofpdf`
- External PDF backend pinned as a module dependency. Backend implementation
	changes and focused regression tests belong in the fork repository.

## Enforced Import Rules

- `internal/templating` must not import:
- `internal/pdfrender`

- `examples/` contains reference profile data and assets, not a Go package or
	a dependency layer.

Go's `internal` package rule prevents external consumers from importing any
internal package. The external-consumer fixture additionally compiles the
supported `csspdf` API from a separate module at the minimum Go version.

## Maintainability Budgets

Default thresholds (can be overridden by env vars):

- Cyclomatic complexity (`MAX_CYCLO`): `12`
- Function length in non-test Go files (`MAX_FUNC_LINES`): `80`
- Go file length (`MAX_FILE_LINES`): `550`

## Local Usage

```sh
go install github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0
scripts/check-maintainability.sh
```

The default `worktree` scope checks modified and untracked project Go files.
CI uses `CHECK_SCOPE=changed` with an explicit base revision. Use
`CHECK_SCOPE=all` to inspect all owned Go files; existing debt above the ratchet
will be reported.

All scopes include root-level Go files when those files are selected. Dependency
source is outside this repository and therefore outside these budgets.

Run the complete build/test/security gate separately:

```sh
scripts/check-quality.sh
```

## Static Analysis Policy

`scripts/check-quality.sh` runs pinned `staticcheck` (default check set) and
`errcheck` on the root module. Unused code, dead stores, and silently discarded
errors fail the gate. `scripts/errcheck-excludes.txt` lists the only functions
whose results may be ignored; each entry carries a reason. Explicit blank
assignments (`_ = f()`) are permitted and mark a deliberate decision; write-side
`Close` calls are not excluded and must be checked. The backend repository owns
its implementation-level analysis and regression gates.
