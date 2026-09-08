#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BENCHTIME="${BENCHTIME:-10x}"
COUNT="${COUNT:-1}"
PROFILE_DIR="${PROFILE_DIR:-/tmp/csspdf-phase5-profile}"

cd "$ROOT_DIR"
mkdir -p "$PROFILE_DIR"

echo "[phase5] Reference environment"
go version
uname -a
if [[ "$(uname -s)" == "Darwin" ]]; then
  sysctl -n machdep.cpu.brand_string 2>/dev/null || true
  sw_vers
fi

echo "[phase5] Workload corpus (benchtime=$BENCHTIME count=$COUNT)"
go test ./docflowpdf -run '^$' -bench '^BenchmarkRenderWorkloads$' -benchtime="$BENCHTIME" -count="$COUNT" -benchmem

echo "[phase5] Long-table heap profile"
go test ./docflowpdf -run '^$' -bench '^BenchmarkRenderWorkloads/long-table$' -benchtime="$BENCHTIME" -count=1 \
  -o="$PROFILE_DIR/docflowpdf.test" \
  -memprofile="$PROFILE_DIR/long-table.mem" -cpuprofile="$PROFILE_DIR/long-table.cpu"
go tool pprof -top -alloc_space -nodecount=15 "$PROFILE_DIR/long-table.mem"

echo "[phase5] Benchmark-process peak RSS"
if [[ "$(uname -s)" == "Darwin" ]]; then
  /usr/bin/time -l go test ./docflowpdf -run '^$' -bench '^BenchmarkRenderWorkloads/long-table$' -benchtime="$BENCHTIME" -count=1 >/dev/null
elif /usr/bin/time -v true >/dev/null 2>&1; then
  /usr/bin/time -v go test ./docflowpdf -run '^$' -bench '^BenchmarkRenderWorkloads/long-table$' -benchtime="$BENCHTIME" -count=1 >/dev/null
else
  echo "peak RSS unavailable: /usr/bin/time does not support -l or -v" >&2
fi
