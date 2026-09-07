# Phase 2 Layout Contract

Phase 2 establishes bounded, persistent document flow. It intentionally changes
documents that previously relied on overlapping sections or content extending
beyond the page boundary.

## Persistent Flow

Normal-flow state now belongs to the layout for the duration of a render.
Consecutive flow sections continue after prior content, including the prior
bottom margin. Explicit page breaks reset the cursor to the next page's content
box. Absolute elements and page-number overlays do not advance or reset normal
flow.

## Geometry

Page dimensions and margins must be finite and non-negative, page dimensions
must be positive, and every first/default page must have a positive content
width and height. Invalid geometry fails before PDF construction.

Absolute blocks and tables must remain inside the physical page. Unsplittable
normal-flow content, such as an image taller than an empty content box, returns
a contextual overflow error. Long text is splittable and continues on later
pages.

## Tables

Column traversal, row measurement, and rendering use the same logical colspan
occupancy. A colspan that exceeds the remaining columns is invalid.

Normal-flow tables paginate between rows. Leading rows originating from
`thead` repeat on every fragment, body-row order is preserved, and each row is
rendered exactly once. A row that cannot fit on a fresh page together with its
repeated headers returns an error; rows are not split internally. Table titles
appear only on the first fragment.

## Text Continuation

Text blocks that exceed the available content height retain the text engine's
original wrapping and line spacing and continue from the next page's content
origin. Top spacing applies to the first fragment and bottom spacing to the
last. A single indivisible text/image band taller than an empty page fails
instead of looping or overflowing.

## Semantic Verification

Layout regressions use `github.com/pdfcpu/pdfcpu` as a pinned test dependency.
Tests independently validate the page tree, extract decoded page content, and
assert page counts, section placement, header repetition, and exact once-only
row/text markers. Raw PDF byte snapshots are not used as layout contracts.