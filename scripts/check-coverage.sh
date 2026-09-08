#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BASELINE_FILE="${COVERAGE_BASELINE_FILE:-$ROOT_DIR/scripts/coverage-baseline.txt}"
PROFILE="${COVERAGE_PROFILE:-$(mktemp "${TMPDIR:-/tmp}/csspdf-coverage.XXXXXX")}"
REMOVE_PROFILE=true
if [[ -n "${COVERAGE_PROFILE:-}" ]]; then
  REMOVE_PROFILE=false
fi
cleanup() {
  if [[ "$REMOVE_PROFILE" == "true" ]]; then
    rm -f "$PROFILE"
  fi
}
trap cleanup EXIT

cd "$ROOT_DIR"

echo "[coverage] Running root-module tests with statement coverage..."
go test ./... -count=1 -covermode=atomic -coverprofile="$PROFILE"

exit_code=0
while read -r package minimum extra; do
  if [[ -z "$package" || "${package#\#}" != "$package" ]]; then
    continue
  fi
  if [[ -z "$minimum" || -n "${extra:-}" ]]; then
    echo "ERROR: invalid coverage baseline row for $package" >&2
    exit 2
  fi

  actual="$(awk -v target="$package" '
    NR == 1 { next }
    {
      file = $1
      sub(/:[0-9].*$/, "", file)
      componentCount = split(file, components, "/")
      packagePath = components[1]
      for (componentIndex = 2; componentIndex < componentCount; componentIndex++) {
        packagePath = packagePath "/" components[componentIndex]
      }
      if (packagePath == target) {
        statements += $2
        if ($3 > 0) {
          covered += $2
        }
      }
    }
    END {
      if (statements == 0) {
        exit 1
      }
      printf "%.1f", covered * 100 / statements
    }
  ' "$PROFILE")" || {
    echo "ERROR: no coverage data for $package" >&2
    exit_code=1
    continue
  }

  if ! awk -v actual="$actual" -v minimum="$minimum" 'BEGIN { exit !(actual + 0 >= minimum + 0) }'; then
    echo "ERROR: $package coverage ${actual}% is below ${minimum}%" >&2
    exit_code=1
  else
    echo "[coverage] $package ${actual}% (floor ${minimum}%)"
  fi
done < "$BASELINE_FILE"

if [[ "$exit_code" -ne 0 ]]; then
  exit "$exit_code"
fi

echo "[coverage] OK"
