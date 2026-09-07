#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

MAX_CYCLO="${MAX_CYCLO:-15}"
MAX_FUNC_LINES="${MAX_FUNC_LINES:-80}"
MAX_FILE_LINES="${MAX_FILE_LINES:-600}"
CHECK_SCOPE="${CHECK_SCOPE:-worktree}"
BASE_REF="${BASE_REF:-}"

GOCYCLO_BIN=""
if command -v gocyclo >/dev/null 2>&1; then
  GOCYCLO_BIN="$(command -v gocyclo)"
elif [[ -x "$(go env GOPATH)/bin/gocyclo" ]]; then
  GOCYCLO_BIN="$(go env GOPATH)/bin/gocyclo"
fi

if [[ -z "$GOCYCLO_BIN" ]]; then
  echo "ERROR: gocyclo is required. Install with:" >&2
  echo "  go install github.com/fzipp/gocyclo/cmd/gocyclo@v0.6.0" >&2
  exit 1
fi

status=0
comparison_ref=""

if [[ "$CHECK_SCOPE" == "worktree" ]] && git rev-parse HEAD >/dev/null 2>&1; then
  comparison_ref="HEAD"
elif [[ "$CHECK_SCOPE" == "changed" ]]; then
  if [[ -n "$BASE_REF" ]] && git cat-file -e "${BASE_REF}^{commit}" >/dev/null 2>&1; then
    comparison_ref="$(git merge-base "$BASE_REF" HEAD)"
  elif git rev-parse HEAD~1 >/dev/null 2>&1; then
    comparison_ref="HEAD~1"
  fi
fi

collect_go_files() {
  if [[ "$CHECK_SCOPE" == "all" ]]; then
    {
      find . -maxdepth 1 -type f -name '*.go'
      find docflowpdf internal cmd -type f -name '*.go'
    } | sort
    return
  fi

  if [[ "$CHECK_SCOPE" == "worktree" ]]; then
    {
      git diff --name-only --diff-filter=ACMRTUXB HEAD -- '*.go' ':!third_party/**'
      git ls-files --others --exclude-standard -- '*.go' ':!third_party/**'
    } | sort -u
    return
  fi

  if [[ -n "$comparison_ref" ]]; then
    git diff --name-only --diff-filter=ACMRTUXB "${comparison_ref}...HEAD" -- '*.go' ':!third_party/**' \
      | sort
    return
  fi

  {
    find . -maxdepth 1 -type f -name '*.go'
    find docflowpdf internal cmd -type f -name '*.go'
  } | sort
}

GO_FILES=()
while IFS= read -r file; do
  if [[ -n "$file" ]]; then
    GO_FILES[${#GO_FILES[@]}]="$file"
  fi
done < <(collect_go_files)

NON_TEST_GO_FILES=()
go_file_count=${#GO_FILES[@]}
if (( go_file_count > 0 )); then
  for file in "${GO_FILES[@]}"; do
    if [[ "$file" != *_test.go ]]; then
      NON_TEST_GO_FILES[${#NON_TEST_GO_FILES[@]}]="$file"
    fi
  done
fi
non_test_go_file_count=${#NON_TEST_GO_FILES[@]}

if (( go_file_count == 0 )); then
  echo "[maintainability] No Go files in selected scope (${CHECK_SCOPE}); checking architecture boundaries only."
fi

baseline_dir=""
cleanup() {
  if [[ -n "$baseline_dir" && -d "$baseline_dir" ]]; then
    rm -rf "$baseline_dir"
  fi
}
trap cleanup EXIT

baseline_files=()
if [[ -n "$comparison_ref" ]] && (( non_test_go_file_count > 0 )); then
  baseline_dir="$(mktemp -d "${TMPDIR:-/tmp}/csspdf-maintainability.XXXXXX")"
  for file in "${NON_TEST_GO_FILES[@]}"; do
    if git cat-file -e "${comparison_ref}:${file}" >/dev/null 2>&1; then
      mkdir -p "$baseline_dir/$(dirname "$file")"
      git show "${comparison_ref}:${file}" > "$baseline_dir/$file"
      baseline_files[${#baseline_files[@]}]="$baseline_dir/$file"
    fi
  done
fi

echo "[maintainability] Checking cyclomatic complexity (max=${MAX_CYCLO})..."
if (( non_test_go_file_count > 0 )); then
  cyclo_out="$($GOCYCLO_BIN -over "$MAX_CYCLO" "${NON_TEST_GO_FILES[@]}" || true)"
  baseline_cyclo_out=""
  if [[ -n "$cyclo_out" ]] && (( ${#baseline_files[@]} > 0 )); then
    baseline_cyclo_out="$($GOCYCLO_BIN -over "$MAX_CYCLO" "${baseline_files[@]}" || true)"
  fi
  while IFS= read -r violation; do
    [[ -z "$violation" ]] && continue
    complexity="${violation%% *}"
    remainder="${violation#* }"
    package_name="${remainder%% *}"
    remainder="${remainder#* }"
    function_name="${remainder%% *}"
    baseline_complexity="$(printf '%s\n' "$baseline_cyclo_out" | awk -v package_name="$package_name" -v function_name="$function_name" '$2 == package_name && $3 == function_name { print $1; exit }')"
    if [[ -z "$comparison_ref" || -z "$baseline_complexity" ]] || (( complexity > baseline_complexity )); then
      echo "$violation"
      status=1
    fi
  done <<< "$cyclo_out"
fi

echo "[maintainability] Checking file length budget (max=${MAX_FILE_LINES})..."
if (( non_test_go_file_count > 0 )); then
  for file in "${NON_TEST_GO_FILES[@]}"; do
    line_count="$(wc -l < "$file" | tr -d ' ')"
    if (( line_count > MAX_FILE_LINES )); then
      baseline_line_count=""
      if [[ -n "$comparison_ref" ]] && git cat-file -e "${comparison_ref}:${file}" >/dev/null 2>&1; then
        baseline_line_count="$(git show "${comparison_ref}:${file}" | wc -l | tr -d ' ')"
      fi
      if [[ -z "$baseline_line_count" ]] || (( line_count > baseline_line_count )); then
        echo "${file}: file too long (${line_count} > ${MAX_FILE_LINES}; baseline=${baseline_line_count:-new})"
        status=1
      fi
    fi
  done
fi

echo "[maintainability] Checking function length budget (max=${MAX_FUNC_LINES})..."
if (( non_test_go_file_count > 0 )); then
  function_length_program='
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
    start_line = FNR
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
    printf "%s\t%d\t%s:%d\n", signature, lines, FILENAME, start_line
      }
      infunc = 0
      depth = 0
      started = 0
      lines = 0
    }
  }
}
'
  func_out="$(awk -v max="$MAX_FUNC_LINES" "$function_length_program" "${NON_TEST_GO_FILES[@]}" || true)"
  baseline_func_out=""
  if [[ -n "$func_out" && -n "${baseline_dir:-}" ]] && (( ${#baseline_files[@]} > 0 )); then
    baseline_func_out="$(awk -v max="$MAX_FUNC_LINES" "$function_length_program" "${baseline_files[@]}" || true)"
  fi
  while IFS=$'\t' read -r signature function_lines location; do
    [[ -z "$signature" ]] && continue
    baseline_function_lines="$(printf '%s\n' "$baseline_func_out" | awk -F '\t' -v signature="$signature" '$1 == signature { print $2; exit }')"
    if [[ -z "$comparison_ref" || -z "$baseline_function_lines" ]] || (( function_lines > baseline_function_lines )); then
      echo "${location}: function too long (${function_lines} > ${MAX_FUNC_LINES}): ${signature}"
		status=1
    fi
  done <<< "$func_out"
fi

echo "[maintainability] Checking architecture boundaries..."
check_forbidden_imports() {
  local scope="$1"
  local pattern="$2"
  local label="$3"

  if [[ ! -d "$scope" ]]; then
    return
  fi

  local out
  out="$(rg -n --glob '*.go' "$pattern" "$scope" || true)"
  if [[ -n "$out" ]]; then
    echo "Architecture violation: ${label}"
    echo "$out"
    status=1
  fi
}

check_forbidden_imports "internal/templating" 'github.com/otuschhoff/csspdf/internal/(invoice|pdfrender|app)' "internal/templating must stay parser/template only"
check_forbidden_imports "internal/flowrender" 'github.com/otuschhoff/csspdf/internal/(invoice|app)' "internal/flowrender must stay adapter-only (no business layer imports)"

if (( status != 0 )); then
  echo "[maintainability] FAILED"
  exit 1
fi

echo "[maintainability] OK"
