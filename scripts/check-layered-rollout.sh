#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

OUT_DIR="output/phase5"
mkdir -p "$OUT_DIR"

echo "[1/3] Running focused regression tests"
go test . ./internal/pdfrender ./internal/pdfdom

echo "[2/3] Rendering layered profile"
go run ./cmd/gen-example layered -o "$OUT_DIR/layered.pdf"

echo "[3/3] Measuring layered-style duplication"
"$ROOT_DIR/scripts/measure-layered-duplication.sh"

layered_size=$(wc -c < "$OUT_DIR/layered.pdf")

cat <<REPORT

Phase 5 rollout summary
- layered example PDF: $OUT_DIR/layered.pdf (${layered_size} bytes)
- focused tests: pass
- duplication metric: see script output above
REPORT
