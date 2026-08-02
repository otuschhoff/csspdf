// Package docflowpdf provides a generic, flow-driven PDF rendering library.
//
// It renders PDF documents from:
//   - HTML template fragments
//   - CSS
//   - flow JSON (section orchestration + payload transforms)
//   - source data (JSON or native Go objects)
//   - optional Go template functions
//
// The package supports file, writer, and in-memory byte outputs.
//
// For migration from legacy single-CSS inputs to layered CSS, helper
// functions are provided:
//   - LegacyCSSSourceAsLayer
//   - MigrateAssetInputLegacyCSSToSingleLayer
package docflowpdf
