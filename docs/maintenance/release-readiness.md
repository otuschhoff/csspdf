# Phase 6 Release Readiness

This record closes the engineering work in Phase 6. It defines sustained gates,
coverage floors, compatibility and release policy, provenance, and the status
of every finding from the original review.

## Supported Matrix and Sustained Gates

The quality workflow tests Go 1.26.6 and Go 1.27.x on Linux, macOS, and Windows
amd64. The minimum version is exact and matches the module directive. The full
root-module race suite runs on Linux. The backend fork runs its own module
quality gates. Root tests include integration fixtures, the pdfcpu semantic PDF
suite, and an external-consumer module that builds and renders from an
unrelated empty working directory.

The security workflow runs bounded fuzzing on pull requests and for one minute
on its weekly schedule. The same schedule performs the pinned reachable-code
vulnerability scan. Workflow actions and analysis tools are version-pinned and
run with read-only repository permissions.

## Coverage Ratchet

`scripts/check-coverage.sh` runs the root suite with atomic statement coverage
and compares each owned package to `scripts/coverage-baseline.txt`. The shared
quality gate invokes it on every supported Go/OS combination. Floors are the
lowest observed value across supported Go/OS cells because instrumentation and
platform-specific code change statement totals. They are package-specific so
high coverage in one package cannot conceal a regression in another.

The coverage baseline was re-recorded after Phase 10 using Go 1.26.6 and Go
1.27.1 profiles. Go coverage instrumentation assigns different statement
boundaries and totals between those toolchains, so each common floor uses the
lowest observed value across the supported matrix rather than a single
toolchain's percentage:

Phase 4 of the root-package structure initiative added `internal/fileout` with
complete behavior coverage. Its 100% floor was recorded when the package was
introduced so the extraction cannot weaken the package-level ratchet.

| Package | Floor |
| --- | ---: |
| `cmd/pdfdump` | 88.9% |
| `cmd/dom-parse` | 74.5% |
| `cmd/gen-example` | 65.3% |
| `csspdf` | 78.6% |
| `internal/fileout` | 100.0% |
| `internal/flowrender` | 67.1% |
| `internal/format` | 57.1% |
| `internal/i18n` | 88.2% |
| `internal/pdfdom` | 76.2% |
| `internal/pdfdump` | 89.5% |
| `internal/pdfrender` | 75.0% |
| `internal/templating` | 92.4% |

Floors may increase with reviewed behavior tests. Lowering one requires an
explicit rationale in the change and updated evidence here. Tests must assert
behavior; wrapper-only tests added solely to move the percentage are rejected.

## Review Finding Closure

| Findings | Status | Permanent evidence |
| --- | --- | --- |
| R01 | Closed | Pinned remote dependencies, clean-checkout and external-consumer gates; `docs/history/phase0-baseline.md` |
| R02-R04 | Closed | Required-content, atomic-output, and currency policy regressions; `docs/guides/migration-v0.2.md` |
| R05-R07 | Closed | Section flow, pagination, colspan, and pdfcpu semantic tests; `docs/architecture/layout.md` |
| R08-R09 | Closed | Exact JSON-number and self-contained i18n tests; `docs/guides/migration-v0.2.md` |
| R10, R14-R15 | Closed | Confined resources, immutable inputs, budgets, cancellation, and fuzz tests; `docs/architecture/security.md` |
| R11 | Closed | Multi-Go/multi-OS quality, race, coverage, maintainability, scheduled fuzz and vulnerability workflows |
| R12-R13 | Closed | Explicit CSS source and layered flow inference regressions; `docs/guides/migration-v0.2.md` |
| R16-R17 | Closed | Public ownership boundaries, structured diagnostics, redaction, and external consumer tests; architecture docs |
| R18 | Closed | Prepared render inputs, measured caches, benchmarks, and race tests; `docs/maintenance/performance.md` |
| R19 | Closed | Executable CSS matrix and support diagnostics; `docs/concepts/layered-css-organization.md` |
| R20 | Closed | Consolidated dumper, supported command tests, utility status, sanitized examples, and `docs/maintenance/provenance.md` |

All P1 and conditional P1 findings in the review have a regression surface and
are closed. No engineering P2 is deferred from R01-R20.

## Remaining Owner Decision

| Decision | Owner | Rationale | Milestone |
| --- | --- | --- | --- |
| Select and add the root repository license | Repository owner | Licensing cannot be inferred from dependency licenses or chosen by automation | Before the first public source or binary release |

This decision blocks public release but does not block internal validation. No
other vulnerability exception, asset exception, or known deferred P2 is
approved.

## Documentation Index

- API compatibility and deprecations: `docs/architecture/api-compatibility.md`
- Security and resource policy: `docs/architecture/security.md`
- HTML/CSS support: `docs/concepts/layered-css-organization.md`
- Troubleshooting: `docs/guides/troubleshooting.md`
- Asset and utility provenance: `docs/maintenance/provenance.md`
- Performance and determinism: `docs/maintenance/performance.md`
- Release procedure: `docs/maintenance/release.md`

Phase 6 establishes repeatable evidence; it does not claim that linting alone
makes the project defect-free or that trusted Go templates are sandboxed.
