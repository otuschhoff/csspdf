# Security Policy

## Supported Surface

Security fixes apply to the current `main` branch. The supported runtime surface
is the `docflowpdf` Go package and the Go commands under `cmd`. Root JavaScript
utilities are retained as unsupported historical tools and are not covered by
the Go dependency or CI policy.

## Current Trust Boundary

csspdf is currently designed for trusted local document authoring. The
following inputs are trusted configuration, not sandboxed data:

- HTML and CSS templates
- flow definitions
- file paths, `io/fs` implementations, image paths, and font registrations
- custom Go template functions and their returned values

Do not allow untrusted users or tenants to control those inputs in the same
process as secrets or sensitive files. Resource lookup can read configured
absolute paths and compatibility search locations outside an asset directory.
The renderer does not currently provide hard limits for input bytes, expanded
templates, DOM nodes, images, pages, output bytes, CPU time, or memory.

Source data values are escaped by Go's `html/template` during normal template
execution, but that does not sandbox custom functions, restrict filesystem
access, or provide resource limits. The renderer does not fetch HTTP resources
itself.

For less-trusted workloads, use a dedicated process or container with a
read-only minimal filesystem, no secrets, restricted network access, bounded
CPU/memory/time, and application-level input limits. A confined resource
resolver and built-in render budgets are planned but are not part of the
current contract.

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