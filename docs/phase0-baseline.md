# Phase 0 Reproducible Baseline

This record closes Phase 0 of the code-quality review. It describes the
repository state and checks established on 2026-09-07.

## Dependency and Toolchain Contract

- Minimum supported toolchain: Go 1.25.13. Earlier Go 1.25 patch releases
  contain reachable standard-library vulnerabilities.
- The root module is self-contained. It does not require a Go workspace or a
  sibling checkout.
- The required gofpdf fork is repository-owned source at
  `third_party/gofpdf`. `third_party/gofpdf/PATCHES.md` records its upstream
  commit, retained scope, local patches, and update procedure.
- Root and nested module manifests must pass `go mod tidy -diff` and
  `go mod verify`.

An export containing only versioned and intended new files passed
`scripts/check-quality.sh` with `GOWORK=off`, no sibling repositories, and
fresh module and build caches. The invoice example also rendered a non-empty
PDF in that environment. The same exported tree passed the quality gate and
invoice smoke render in Linux; the supported Go 1.25 patch line also passed the
gate on macOS arm64.

## Hermetic Font Integration

The UTF-8 integration test uses Go Regular from the pinned
`golang.org/x/image/font/gofont/goregular` package. It writes the font bytes to
a test-owned temporary directory and does not depend on example assets or
private modules. The test asserts Type0 emission, an explicit ToUnicode CMap,
and mappings for the German characters it renders.

## Vulnerability Baseline

The initial reachable-code scan used
`golang.org/x/vuln/cmd/govulncheck@v1.7.0` on 2026-09-07. Earlier dependency
resolution identified reachable findings through `golang.org/x/net`; the
module was upgraded to v0.56.0 and the module graph was tidied. Revalidation
under Go 1.25.0 then identified six reachable standard-library findings fixed
in later Go 1.25 patch releases, so the supported and CI baseline was raised to
Go 1.25.13. The final scan under Go 1.25.13 reported
`No vulnerabilities found.`

This result is a point-in-time baseline, not a claim that the project is free
of undiscovered vulnerabilities. CI reruns the pinned scan on Linux. Tool or
database upgrades should be reviewed and pinned deliberately.

## Required Gates

`scripts/check-quality.sh` is the shared local/CI entry point for module
tidiness and checksums, formatting, builds, vet, root and nested tests, and the
vulnerability scan. CI runs the non-platform-specific quality gates on Linux
and macOS with Go 1.25.13 and least-privilege permissions.

`scripts/check-maintainability.sh` is a changed-code ratchet. It includes public
and internal owned Go code, supports local worktree and CI diff scopes, and
uses pinned `gocyclo` v0.6.0.