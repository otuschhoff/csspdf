# Local gofpdf Source

This directory contains the root Go package from `github.com/otuschhoff/gofpdf`
commit `0a62bf7bfe71a6aeaa4b51db43f638409971c52b`. The module is kept in this
repository because csspdf depends on template-gradient APIs from that commit
that are not present in the fork's published `master` branch. Demo binaries and
assets, optional subpackages, and fixture-coupled upstream tests are
intentionally excluded. `csspdf_patch_test.go` covers the local patch set;
csspdf's renderer tests cover the imported template and gradient APIs.

The source also includes two focused changes that were uncommitted in the
source checkout when imported:

1. `Fpdf.CurrentFontIsUTF8` exposes the active font encoding so csspdf can use
   rune-safe wrapping for UTF-8 fonts and byte-oriented wrapping for core
   fonts.
2. UTF-8 font emission writes explicit, used-glyph `beginbfchar` entries in
   each ToUnicode CMap and copies font usage maps before output.

A third patch set applies `staticcheck` SA-class (correctness) fixes found by
the csspdf quality gate. Style-class findings are intentionally not applied to
this imported source; the gate runs `staticcheck -checks 'SA*'` here.

3. `SetTextSpotColor` compared the text color string with itself, so the
   color flag was never set for spot text colors; it now compares fill and
   text as the other color setters do.
4. `sliceUncompress` deferred `Close` on a reader before checking the
   `zlib.NewReader` error, which would panic on invalid input.
5. `parseCMAPTable` overwrote its parameter before use; the parameter is
   removed and `parseTables` no longer threads the unused value.
6. `ttfparser.go` uses `io.SeekStart`/`io.SeekCurrent` instead of the
   deprecated `os.SEEK_*` constants.
7. Four index-free `range []rune(s)` loops range over the string directly;
   rune sequences are identical.
8. Deprecated `io/ioutil` calls use their identical `io` and `os`
   replacements required by the Go 1.22 static-analysis baseline.

The nested module now declares Go 1.22 language semantics, matching the oldest
release with per-iteration loop variables while remaining independent of the
root module's newer minimum. The vestigial `replace gofpdf => ./` directive was
removed. All 126 range loops were reviewed; none launches a goroutine or defers
a closure that captures its loop variables, so the semantic change does not
alter current behavior. Nested formatting, vet, tests, and the root quality
gate validate the updated directive.

The upstream MIT and ISC license files are preserved. Update this source only
as an explicit dependency change: record the new immutable base revision,
reapply and document local changes, run `go test ./...` in this directory, run the csspdf
full suite, and validate a clean standalone checkout.