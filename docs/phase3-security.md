# Phase 3 Security and Operational Contract

Phase 3 provides enforceable resource and availability boundaries. It does not
turn Go templates or in-process rendering into a hostile-code sandbox.

## Resource Modes

`WithConfinedResourceRoot(root)` resolves file-backed sources, images, i18n,
and fonts beneath one authorized root. Absolute paths, parent traversal,
symlink escape, directories, and unsupported image/font files fail. Authorized
files are read once and images/fonts are passed to gofpdf as bytes.

Legacy APIs use trusted host-file access for compatibility. Applications should
use `WithTrustedFileAccess()` when that unrestricted policy is intentional.
Caller-provided `io/fs` values remain caller-owned trust boundaries.

## Render Budgets

`RenderLimits` has nonzero defaults for source and expanded-template bytes,
compressed image bytes, decoded pixels, cumulative nodes and rows, nesting
depth, pages, and output bytes. Zero selects the default; negative limits fail.
Over-limit rendering returns `*BudgetError` and file output remains atomic.

## Cancellation

`RenderContext`, `RenderWithInputContext`, `RenderToFileContext`,
`RenderToWriterContext`, and `RenderToBytesContext` accept cancellation.
Checks occur before preparation, while reading and expanding sources, between
flow elements and pages, and before output. Custom functions and some backend
operations are not preemptible; hard deadlines require process isolation.

## PDF Inspection

The canonical PDF dumper limits input bytes and per-stream Flate expansion and
writes to an injected `io.Writer`. Decompression errors are returned. Both Go
commands delegate to this implementation.

## Service Deployment

Services must set a confined root and explicit workload-specific limits, bound
worker concurrency and queue depth, and isolate rendering from credentials and
unnecessary network/filesystem access. Use OS/container CPU, memory, and time
limits for hostile workloads.