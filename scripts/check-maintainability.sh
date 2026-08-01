#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

MAX_CYCLO="${MAX_CYCLO:-15}"
MAX_FUNC_LINES="${MAX_FUNC_LINES:-80}"
MAX_FILE_LINES="${MAX_FILE_LINES:-600}"
CHECK_SCOPE="${CHECK_SCOPE:-changed}"
BASE_REF="${BASE_REF:-}"

GOCYCLO_BIN=""
if command -v gocyclo >/dev/null 2>&1; then
  GOCYCLO_BIN="$(command -v gocyclo)"
elif [[ -x "$(go env GOPATH)/bin/gocyclo" ]]; then
  GOCYCLO_BIN="$(go env GOPATH)/bin/gocyclo"
fi

if [[ -z "$GOCYCLO_BIN" ]]; then
  echo "ERROR: gocyclo is required. Install with:" >&2
  echo "  go install github.com/fzipp/gocyclo/cmd/gocyclo@latest" >&2
  exit 1
fi

status=0

collect_go_files() {
  if [[ "$CHECK_SCOPE" == "all" ]]; then
    find internal cmd -type f -name '*.go' | sort
    return
  fi

  local range=""
  if [[ -n "$BASE_REF" ]] && git cat-file -e "${BASE_REF}^{commit}" >/dev/null 2>&1; then
    range="${BASE_REF}...HEAD"
  elif git rev-parse HEAD~1 >/dev/null 2>&1; then
    range="HEAD~1...HEAD"
  fi

  if [[ -n "$range" ]]; then
    git diff --name-only --diff-filter=ACMRTUXB "$range" -- internal cmd \
      | rg '\.go$' \
      | sort
    return
  fi

  find internal cmd -type f -name '*.go' | sort
}

mapfile -t GO_FILES < <(collect_go_files)
mapfile -t NON_TEST_GO_FILES < <(printf '%s\n' "${GO_FILES[@]}" | rg -v '_test\.go$' || true)

if [[ ${#GO_FILES[@]} -eq 0 ]]; then
  echo "[maintainability] No Go files in selected scope (${CHECK_SCOPE}); checking architecture boundaries only."
fi

echo "[maintainability] Checking cyclomatic complexity (max=${MAX_CYCLO})..."
if [[ ${#NON_TEST_GO_FILES[@]} -gt 0 ]]; then
  cyclo_out="$($GOCYCLO_BIN -over "$MAX_CYCLO" "${NON_TEST_GO_FILES[@]}" || true)"
  if [[ -n "$cyclo_out" ]]; then
    echo "$cyclo_out"
    status=1
  fi
fi

echo "[maintainability] Checking file length budget (max=${MAX_FILE_LINES})..."
for file in "${GO_FILES[@]}"; do
  line_count="$(wc -l < "$file" | tr -d ' ')"
  if (( line_count > MAX_FILE_LINES )); then
    echo "${file}: file too long (${line_count} > ${MAX_FILE_LINES})"
    status=1
  fi
done

echo "[maintainability] Checking function length budget (max=${MAX_FUNC_LINES})..."
if [[ ${#NON_TEST_GO_FILES[@]} -gt 0 ]]; then
  func_out="$(awk -v max="$MAX_FUNC_LINES" '
function count_char(s, c,    i, n) {
  n = 0
  for (i = 1; i <= length(s); i++) {
    if (substr(s, i, 1) == c) n++
  }
  return n
}
{
  if (!infunc && $0 ~ /^func[[:space:]]/) {
    infunc = 1
    start_line = NR
    signature = $0
    depth = 0
    started = 0
    lines = 0
  }

  if (infunc) {
    lines++
    opens = count_char($0, "{")
    closes = count_char($0, "}")
    if (opens > 0) started = 1
    depth += opens - closes

    if (started && depth <= 0) {
      if (lines > max) {
        printf "%s:%d: function too long (%d > %d): %s\n", FILENAME, start_line, lines, max, signature
      }
      infunc = 0
      depth = 0
      started = 0
      lines = 0
    }
  }
}
' "${NON_TEST_GO_FILES[@]}" || true)"
  if [[ -n "$func_out" ]]; then
    echo "$func_out"
    status=1
  fi
fi

echo "[maintainability] Checking architecture boundaries..."
check_forbidden_imports() {
  local scope="$1"
  local pattern="$2"
  local label="$3"

  local out
  out="$(rg -n --glob '*.go' "$pattern" "$scope" || true)"
  if [[ -n "$out" ]]; then
    echo "Architecture violation: ${label}"
    echo "$out"
    status=1
  fi
}

check_forbidden_imports "internal/domain" 'github.com/otuschhoff/go-dom2pdf/internal/(invoice|pdfrender|templateflow|app|template)' "internal/domain must not import rendering/orchestration/template layers"
check_forbidden_imports "internal/templating" 'github.com/otuschhoff/go-dom2pdf/internal/(invoice|pdfrender|app)' "internal/templating must stay parser/template only"
check_forbidden_imports "internal/flowrender" 'github.com/otuschhoff/go-dom2pdf/internal/(invoice|app)' "internal/flowrender must stay adapter-only (no business layer imports)"

if (( status != 0 )); then
  echo "[maintainability] FAILED"
  exit 1
fi

echo "[maintainability] OK"
