# API Compatibility and Deprecations

## Release Line

csspdf is currently pre-v1 and has no published release tags. Until v1.0.0,
minor releases may contain documented compatibility changes when required for
correctness or security. Patch releases must remain backward compatible within
the current minor line.

The supported Go API is the exported surface of
`github.com/otuschhoff/csspdf/docflowpdf`. Packages below `internal/`, command
implementation details, examples, and repository scripts are not public Go
APIs. Supported commands and their exit-code contracts are listed in the
README.

The root `pdfdump.go` command is the single supported PDF inspection entry
point. The duplicate `gen-example dump-pdf` subcommand was removed before a
stable CLI release; `gen-example` remains focused on rendering repository
examples.

Every intentional public behavior change must include:

- a regression test for the new contract;
- a migration note in `docs/phase1-migration.md` or release notes;
- a compatibility classification in the release checklist; and
- a major-version decision once the project reaches v1.

## Deprecation Policy

Public deprecations use the Go `Deprecated:` doc-comment convention and name a
replacement. A deprecated public API remains available for at least the next
minor release unless retaining it creates a confirmed security or correctness
hazard. Removal must be announced in release notes and requires either a major
release or an explicit pre-v1 compatibility notice.

Current public compatibility path:

| API | Replacement | Removal status |
| --- | --- | --- |
| `RenderInput.WarningWriter` and `WithWarningWriter` | `RenderInput.Logger` and `WithLogger` | Deprecated in the first tagged release (v0.1.0); retained through v0.2.x; removed in v0.3.0 |
| `RenderInput.AllowPartialRender` and `WithLegacyPartialRendering` | Strict rendering, the default | Deprecated in the first tagged release (v0.1.0); retained through v0.2.x; removed in v0.3.0 |
| `FuncMapFactory` | `FuncMapFactoryEx` | Supported legacy hook, not deprecated; no removal release scheduled |

Every deprecated public symbol carries a Go `Deprecated:` comment naming its
replacement and removal release, so `staticcheck` (SA1019) flags external
callers. There are currently no deprecated symbols under `internal/`; the
quality gate's unused-code check rejects deprecated internal wrappers that have
no remaining callers.

## Supported Toolchains and Platforms

The minimum supported toolchain is Go 1.26.6. CI tests Go 1.26.6 and Go 1.27.x
on current GitHub-hosted Linux, macOS, and Windows amd64 runners. New Go release
lines enter the matrix after a green quality and race run. A minimum-version
change requires vulnerability or dependency evidence, documentation, and a
release note. File replacement and path policy follow host OS behavior
documented in `docs/phase1-migration.md` and `docs/phase3-security.md`.
