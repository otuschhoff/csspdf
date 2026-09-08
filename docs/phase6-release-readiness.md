# Phase 6 Release Readiness

This record closes the engineering work in Phase 6. It defines sustained gates,
coverage floors, compatibility and release policy, provenance, and the status
of every finding from the original review.

## Supported Matrix and Sustained Gates

The quality workflow tests Go 1.26.6 and Go 1.27.x on Linux and macOS. The
minimum version is exact and matches the module directive. The full root and
nested-module race suite runs on Linux. Root tests include integration fixtures,
the pdfcpu semantic PDF suite, and an external-consumer module that builds and
renders from an unrelated empty working directory.

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

The Phase 6 baseline was re-recorded in Phase 7 after dead-code removal and
the invoice example's deletion changed statement totals. Values are the minimum
across Linux/macOS at Go 1.26.6 and Go 1.27.1:

| Package | Floor |
| --- | ---: |
| root diagnostic command | 66.7% |
| `cmd/dom-parse` | 74.5% |
| `cmd/gen-example` | 48.0% |
| `docflowpdf` | 73.6% |
| `internal/flowrender` | 66.7% |
| `internal/format` | 57.1% |
| `internal/i18n` | 88.2% |
| `internal/pdfdom` | 48.9% |
| `internal/pdfdump` | 25.2% |
| `internal/pdfrender` | 63.4% |
| `internal/templating` | 46.3% |

Floors may increase with reviewed behavior tests. Lowering one requires an
explicit rationale in the change and updated evidence here. Tests must assert
behavior; wrapper-only tests added solely to move the percentage are rejected.

## Review Finding Closure

| Findings | Status | Permanent evidence |
| --- | --- | --- |
| R01 | Closed | Self-contained root/nested modules, clean-checkout and external-consumer gates; `docs/phase0-baseline.md` |
| R02-R04 | Closed | Required-content, atomic-output, and currency policy regressions; `docs/phase1-migration.md` |
| R05-R07 | Closed | Section flow, pagination, colspan, and pdfcpu semantic tests; `docs/phase2-layout.md` |
| R08-R09 | Closed | Exact JSON-number and self-contained i18n tests; `docs/phase1-migration.md` |
| R10, R14-R15 | Closed | Confined resources, immutable inputs, budgets, cancellation, and fuzz tests; `docs/phase3-security.md` |
| R11 | Closed | Multi-Go/multi-OS quality, race, coverage, maintainability, scheduled fuzz and vulnerability workflows |
| R12-R13 | Closed | Explicit CSS source and layered flow inference regressions; `docs/phase1-migration.md` |
| R16-R17 | Closed | Public ownership boundaries, structured diagnostics, redaction, and external consumer tests; architecture docs |
| R18 | Closed | Prepared render inputs, measured caches, benchmarks, and race tests; `docs/phase5-performance.md` |
| R19 | Closed | Executable CSS matrix and support diagnostics; `docs/concepts/layered-css-organization.md` |
| R20 | Closed | Consolidated dumper, supported command tests, utility status, sanitized examples, and `docs/provenance.md` |

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

- API compatibility and deprecations: `docs/api-compatibility.md`
- Security and resource policy: `docs/phase3-security.md`
- HTML/CSS support: `docs/concepts/layered-css-organization.md`
- Troubleshooting: `docs/troubleshooting.md`
- Asset and utility provenance: `docs/provenance.md`
- Performance and determinism: `docs/phase5-performance.md`
- Release procedure: `docs/release.md`

Phase 6 establishes repeatable evidence; it does not claim that linting alone
makes the project defect-free or that trusted Go templates are sandboxed.
