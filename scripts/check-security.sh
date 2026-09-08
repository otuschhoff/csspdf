#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
FUZZ_TIME="${FUZZ_TIME:-5s}"

cd "$ROOT_DIR"

go test ./docflowpdf -run '^$' -fuzz '^FuzzDecodeJSONAndFlowValidation$' -fuzztime="$FUZZ_TIME"
go test ./docflowpdf -run '^$' -fuzz '^FuzzRenderHTMLDoesNotPanic$' -fuzztime="$FUZZ_TIME"
go test ./internal/pdfdom -run '^$' -fuzz '^FuzzParseHTMLDocFlow$' -fuzztime="$FUZZ_TIME"
go test ./internal/pdfdump -run '^$' -fuzz '^FuzzDecompressFlateBounded$' -fuzztime="$FUZZ_TIME"