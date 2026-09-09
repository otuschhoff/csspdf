# Phase 1 Compatibility and Migration

For the complete release summary, see the
[v0.2.0 release notes](../releases/v0.2.0.md).

Phase 1 makes input, rendering, and output failures explicit. These changes are
intentional compatibility breaks for callers that relied on incomplete output
or permissive configuration.

## v0.2 Package Path

Version v0.2.0 moves the public package to the module root. Replace:

```go
import "github.com/otuschhoff/csspdf/docflowpdf"
```

with:

```go
import "github.com/otuschhoff/csspdf"
```

Replace `docflowpdf.` qualifiers with `csspdf.`. The exported API behavior is
otherwise unchanged. The old package path is not retained because all known
consumers are migrated together before the v0.2.0 release.

## Strict Rendering

Rendering now fails by default when a required template, template map value,
image, reusable template, running-footer child, or other rendered element
cannot be produced. Legacy HTML and layered HTML follow the same policy.

For a temporary migration window, callers can set
`RenderInput.AllowPartialRender` or pass
`WithLegacyPartialRendering(true)`. This restores warning-and-continue behavior
for recoverable failures and may produce an incomplete PDF. It does not suppress
configuration, i18n macro, table, or other fatal errors. New integrations
should not enable it.

The example CLI returns exit status `1` when rendering fails and `2` for command
usage errors.

## Atomic File Output

`RenderToFile` first renders the complete PDF in memory, then writes a temporary
file in the destination directory, closes it, and renames it over the requested
path. Render, write, close, and rename failures preserve an existing
destination and remove the temporary file.

- Replacements preserve the existing destination's permission bits.
- New files use the owner-only `0600` mode created by `os.CreateTemp`.
- Replacing a symlink replaces the link itself; it does not overwrite the
  symlink target.
- Atomic replacement follows the host `os.Rename` contract. On a platform that
  cannot replace an existing file, the operation returns an error and preserves
  the destination.
- Files and directories are not synced. The operation protects against normal
  process errors but does not promise persistence across power loss or kernel
  failure.
- `RenderToWriter` cannot undo bytes accepted by a caller-provided writer.

## Numbers and Currency

Dynamic JSON values now decode with `json.Decoder.UseNumber`. Code that asserted
`float64` for every JSON number must accept `json.Number`; use `String` or
`Int64` for exact integer identifiers and `Float64` only when binary floating
point is appropriate. Default numeric helpers accept `json.Number` and native
integer and floating-point values.

Currency output now rounds before splitting integer and fractional digits.
JPY uses zero minor units, BHD and KWD use three, and all other currency codes
use two. Non-finite values are rendered as `NaN`, `+Inf`, or `-Inf`; values that
round to zero do not retain a negative sign. This formatting policy does not
introduce an exact-decimal financial API.

## Configuration and Defaults

- `i18n.New` no longer searches the current or parent directories. Configure
  `I18nSource`, or use `AssetBaseDir` when a profile-owned `i18n.json` is
  intended. Generic number separators are available without a file.
- An unset legacy CSS source remains valid when CSS layers are present. An
  explicitly configured missing or unreadable legacy source is an error.
- Flow defaults are inferred from parsed template definitions across legacy or
  layered HTML. Text that merely resembles a definition is ignored, and layers
  retain sequential override semantics.
- Unknown flow JSON fields are rejected. Runtime and static payload target
  paths must not conflict or overwrite `Source`, `Payload`, `page`, `locale`, or
  `i18n`.
- Rendering normalizes private copies of mutable flow data, so one input can be
  reused concurrently without defaults mutating caller-owned configuration.