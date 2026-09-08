# Security Policy

## Supported Surface

Security fixes apply to the current `main` branch. The supported runtime surface
is the `docflowpdf` Go package and the Go commands under `cmd`. Root JavaScript
utilities are retained as unsupported historical tools and are not covered by
the Go dependency or CI policy.

## Trust Boundary

HTML/CSS templates, flow definitions, custom template functions, and
caller-provided `io/fs` implementations are trusted executable configuration,
not sandboxed tenant data. Source values are escaped by `html/template`, but
escaping does not make templates or custom functions safe.

`WithConfinedResourceRoot` confines relative template, CSS, JSON, image, i18n,
and font resources with `os.Root`. It rejects absolute/traversal paths,
symlink escape, non-regular files, and unsupported image/font types. The
default and `WithTrustedFileAccess` preserve unrestricted host-file behavior
for trusted local authoring and must not be presented as confinement.

`RenderLimits` bounds source and expanded-template bytes, compressed image
bytes, decoded pixels, cumulative nodes/rows, nesting depth, pages, and output
bytes. Context-aware APIs stop at preparation, source/template, layout/page,
and emission checkpoints. They cannot preempt arbitrary custom functions or
all backend operations and therefore do not provide hard CPU/memory isolation.

For hostile or tenant workloads, use bounded worker concurrency and queue
depth plus a dedicated process or container with a read-only minimal
filesystem, no secrets, restricted network access, and OS-level CPU, memory,
and time limits. The renderer does not fetch HTTP resources itself.

## Vulnerability Exceptions

The pinned `govulncheck` gate must pass by default. A temporary exception must
record the advisory, reachable-call assessment, compensating control, owner,
expiry date, and removal milestone. Exceptions are reviewed before expiry and
must not be represented as an absence of vulnerability.

## Dependency and CI Policy

- Go 1.25.13 is the minimum supported toolchain.
- Root dependencies are pinned by `go.mod` and `go.sum`.
- The repository-owned `third_party/gofpdf` source has provenance and local
  changes documented in its `PATCHES.md`.
- CI builds, vets, tests, checks formatting/module tidiness, and runs pinned
  `govulncheck` analysis.

Report suspected vulnerabilities privately through the repository host's
security-advisory mechanism. Include affected versions, reproduction steps,
impact, and any known mitigation. Avoid including secrets or sensitive source
documents in a report.