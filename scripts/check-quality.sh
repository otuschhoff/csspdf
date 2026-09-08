#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
GOVULNCHECK_VERSION="${GOVULNCHECK_VERSION:-v1.7.0}"
STATICCHECK_VERSION="${STATICCHECK_VERSION:-v0.8.1}"
ERRCHECK_VERSION="${ERRCHECK_VERSION:-v1.20.0}"
RUN_VULN_CHECK="${RUN_VULN_CHECK:-true}"

cd "$ROOT_DIR"

check_formatting() {
  local unformatted
  unformatted="$(find . \
    -path './.git' -prune -o \
    -type f -name '*.go' -print \
    | sort \
    | xargs gofmt -l)"
  if [[ -n "$unformatted" ]]; then
    echo "ERROR: gofmt is required for:" >&2
    echo "$unformatted" >&2
    return 1
  fi
}

echo "[quality] Checking module manifests..."
go mod tidy -diff
go mod verify
(
  cd third_party/gofpdf
  go mod tidy -diff
  go mod verify
)

echo "[quality] Checking formatting..."
check_formatting

echo "[quality] Building all root-module packages..."
go build ./...

echo "[quality] Running static analysis..."
go vet ./...
(
  cd third_party/gofpdf
  go vet ./...
)

echo "[quality] Running staticcheck ${STATICCHECK_VERSION} and errcheck ${ERRCHECK_VERSION}..."
go run "honnef.co/go/tools/cmd/staticcheck@${STATICCHECK_VERSION}" ./...
go run "github.com/kisielk/errcheck@${ERRCHECK_VERSION}" \
  -exclude scripts/errcheck-excludes.txt -ignoretests ./...
go run ./scripts/errorwrapcheck ./...
(
  # Imported backend source: correctness (SA) checks only; style checks would
  # rewrite upstream code beyond the documented patch set.
  cd third_party/gofpdf
  go run "honnef.co/go/tools/cmd/staticcheck@${STATICCHECK_VERSION}" -checks 'SA*' ./...
)

echo "[quality] Running all tests..."
scripts/check-coverage.sh
(
  cd third_party/gofpdf
  go test ./... -count=1
)

if [[ "$RUN_VULN_CHECK" == "true" ]]; then
  echo "[quality] Scanning reachable dependencies with govulncheck ${GOVULNCHECK_VERSION}..."
  go run "golang.org/x/vuln/cmd/govulncheck@${GOVULNCHECK_VERSION}" ./...
fi

echo "[quality] OK"