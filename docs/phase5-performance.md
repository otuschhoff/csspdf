# Phase 5 Performance and Determinism

This record defines the measured performance baseline and determinism contract
for Phase 5. The numbers are observations on one reference runner, not latency
promises for other machines or documents.

## Reference Runner

- Date: 2026-09-08
- OS: macOS 26.6.2 (Darwin 25.6.0, arm64)
- CPU: Apple M1 Max
- Toolchain: Go 1.27.1
- Comparison base: the commit `feat: complete ownership and diagnostics phase`
  (end of Phase 4), reproducible by re-running the command below at that
  subject in history
- Command: `BENCHTIME=10x COUNT=5 scripts/benchmark-phase5.sh`

The benchmark runner also writes CPU and heap profiles under
`/tmp/csspdf-phase5-profile` by default. The long-table benchmark process
reported a maximum resident set size of 149,061,632 bytes (about 142 MiB),
including the Go test harness. Its allocation profile sampled 146.96 MiB over
10 measured renders. Per-render heap allocation is the more useful comparison
and is reported as `B/op` below.

## Workload Corpus

The corpus in `docflowpdf/performance_test.go` is generated in test-owned
memory and temporary directories. It does not depend on private files,
checked-in output PDFs, or system fonts.

| Workload | Contract exercised | Pages | Output bytes |
| --- | --- | ---: | ---: |
| small-letter | simple text and contextual date helper | 1 | 1,320 |
| layered-multi-section | two HTML layers, two CSS layers, four sections | 1 | 1,593 |
| long-table | 200 rows, automatic widths, pagination | 5 | 11,322 |
| images | generated PNG reused four times | 1 | 1,525 |
| UTF-8 | German and Latin-extended text | 1 | 1,514 |
| concurrent-independent | parallel independent layered renders | 1 | 1,593 |

## Current Baseline

These values are medians from five 10-iteration reference runs.

| Workload | ns/op | B/op | allocs/op |
| --- | ---: | ---: | ---: |
| small-letter | 554,783 | 970,761 | 1,821 |
| layered-multi-section | 1,033,292 | 1,263,076 | 4,458 |
| long-table | 15,720,500 | 13,573,465 | 183,053 |
| images | 971,100 | 1,020,777 | 1,498 |
| UTF-8 | 705,738 | 1,051,208 | 1,810 |
| concurrent-independent | 862,812 | 1,319,833 | 4,539 |

## Measured Improvements

Per-render template variants are compiled once for each distinct function-map
signature. CSS declarations and selectors are compiled once per render and
reused across main sections and page-number overlays. Templates are cloned and
bound to current functions for each execution, preserving locale, clock, and
custom factory behavior. There is no process-global render cache.

On the reference runner, the four-section workload moved from 4,930 to 4,440
allocations per render (about 10% fewer) with identical page count and output
size. Single-section documents may pay a small preparation/clone overhead; the
optimization targets repeated sections and page overlays.

The long-table allocation profile showed repeated color parsing and text
encoding as the dominant removable costs. Per-renderer, document-bounded
caches and allocation-free hex parsing changed the 200-row workload from:

| Metric | Before | After | Change |
| --- | ---: | ---: | ---: |
| latency | 29.88 ms/op | 15.72 ms/op | -47% |
| heap allocation | 26.99 MB/op | 13.57 MB/op | -50% |
| allocations | 309,016/op | 183,053/op | -41% |

Both runs produced five pages and 11,322 PDF bytes. The post-change allocation
profile no longer lists color parsing or Latin-1 encoding among its leading
allocators. Width-plan caching was deliberately not added because the profile
did not show it as a leading cost.

## Regression Budget

For performance-sensitive changes, run the corpus with `COUNT=5` on an idle
reference runner and compare medians. Investigate when either `ns/op` or
`B/op` regresses by more than 20% for two consecutive runs. Page count and
output size must remain unchanged unless the change intentionally alters PDF
semantics. Timing is not a blocking CI gate because shared-runner variance can
exceed small changes; allocations and semantic tests remain stable gates.

The measured corpus covers up to 200 table rows and five generated pages. It
is a supported regression envelope, not the library's hard resource ceiling.
Applications must configure `RenderLimits` for their workload; defaults cap
source and output bytes, nodes, depth, rows, images, and pages as documented by
`DefaultRenderLimits`.

## Determinism Contract

Without `RenderInput.Now`, rendering provides semantic stability but PDF bytes
may differ because metadata uses the current time. With a fixed `Now`, the
same deterministic templates, data, resources, options, and custom functions
produce byte-identical PDFs. PDF resource catalogs are sorted before object
IDs are assigned. Tests verify identity both within one process and across
independent processes, including layered multi-section and table layouts. The
fixed time controls template function context plus PDF creation and
modification metadata.
