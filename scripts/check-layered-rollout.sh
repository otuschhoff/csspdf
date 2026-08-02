#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

OUT_DIR="output/phase5"
mkdir -p "$OUT_DIR"

echo "[1/4] Running focused regression tests"
go test ./docflowpdf ./internal/pdfrender ./internal/pdfdom

echo "[2/4] Rendering legacy invoice profile"
go run ./cmd/gen-example invoice -o "$OUT_DIR/invoice-legacy.pdf"

echo "[3/4] Rendering layered profile"
go run ./cmd/gen-example layered -o "$OUT_DIR/layered.pdf"

echo "[4/4] Measuring layered-style duplication"
"$ROOT_DIR/scripts/measure-layered-duplication.sh"

legacy_size=$(wc -c < "$OUT_DIR/invoice-legacy.pdf")
layered_size=$(wc -c < "$OUT_DIR/layered.pdf")

cat <<REPORT

Phase 5 rollout summary
- legacy invoice PDF: $OUT_DIR/invoice-legacy.pdf (${legacy_size} bytes)
- layered example PDF: $OUT_DIR/layered.pdf (${layered_size} bytes)
- focused tests: pass
- duplication metric: see script output above
REPORT
