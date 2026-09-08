# Release Procedure

csspdf has no published release tags yet. This procedure governs the first
pre-v1 release and subsequent releases.

## Release Blockers

A public release must not be created until all of the following are true:

- the repository owner has selected and added a root source license;
- every distributed asset passes `docs/provenance.md` requirements;
- the working tree is clean and generated `output/` or `bin/` files are absent;
- all P1 findings are closed by regression tests;
- remaining P2 risks have an owner, rationale, and milestone;
- the supported Go/OS quality matrix and race job are green;
- the scheduled security workflow has a recent successful run; and
- release notes describe compatibility changes and migrations.

The missing root license is the only known policy blocker after Phase 6. It is
owned by the repository owner and targeted for the first public release. It
cannot be resolved by inferring a dependency license.

## Versioning

Use semantic version tags (`v0.x.y` before API stabilization). Patch releases
within a minor line are backward compatible. Pre-v1 minor releases may include
documented compatibility changes under `docs/api-compatibility.md`. After
v1.0.0, breaking public API changes require a major release.

Do not use the hard-coded example CLI version as the release source of truth.
The annotated Git tag is authoritative until automated version injection is
introduced.

## Candidate Validation

From a clean checkout of the candidate commit:

```sh
GOTOOLCHAIN=go1.26.6 scripts/check-quality.sh
go test -race ./... -count=1
(cd third_party/gofpdf && go test -race ./... -count=1)
scripts/check-security.sh
scripts/check-layered-rollout.sh
CHECK_SCOPE=all scripts/check-maintainability.sh
BENCHTIME=10x COUNT=5 scripts/benchmark-phase5.sh
```

Confirm GitHub Actions passes the complete Linux/macOS and Go-version matrix.
The normal test suite includes the pdfcpu semantic PDF checks and builds/runs
the README-style external consumer from an unrelated clean directory. Review
benchmark medians under the budget in `docs/phase5-performance.md`; timing is a
human-reviewed reference-runner signal, not a shared-CI promise.

Inspect the release tree with `git status --short`, `git diff --check`, and
`git ls-files`. Confirm no private source data, unknown-provenance assets,
profiles, test binaries, generated PDFs, or editor files are included.

## Publish

1. Update release notes with changes, migration actions, deprecations, security
   impact, supported Go versions, and known P2 risks.
2. Re-run candidate validation on the exact release commit.
3. Create an annotated tag: `git tag -a v0.x.y -m "csspdf v0.x.y"`.
4. Push the commit and tag; do not publish if required workflows fail.
5. Build source archives from tracked files at the tag, not from the working
   directory.
6. Verify the archive in a fresh directory with `GOWORK=off` and fresh Go
   caches.
7. Record checksums and the workflow run URLs in the release notes.

## Vulnerability Exceptions

No vulnerability exceptions are currently approved. A future exception must
name the advisory, affected reachability, compensating controls, owner, expiry
date, and removal milestone. Expired exceptions block release. The pinned scan
must still run and the exception must be reviewed against its output.

## Rollback

Tags are immutable. If a release is defective, publish a new patch release or,
for a severe security issue, mark the release withdrawn and document the safe
replacement. Never move or silently replace an existing tag.
