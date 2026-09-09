#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
status=0
inventory_file="$(mktemp "${TMPDIR:-/tmp}/csspdf-doc-inventory.XXXXXX")"
index_file="$(mktemp "${TMPDIR:-/tmp}/csspdf-doc-index.XXXXXX")"

cleanup() {
  rm -f "$inventory_file" "$index_file"
}
trap cleanup EXIT

cd "$ROOT_DIR"

echo "[docs] Checking local Markdown links..."

while IFS=$'\t' read -r source line target; do
  case "$target" in
    ""|\#*|//*|mailto:*|http:*|https:*)
      continue
      ;;
  esac

  target="${target#<}"
  target="${target%>}"
  target="${target%%#*}"
  target="${target%%\?*}"
  [[ -z "$target" ]] && continue

  if [[ "$target" == docs/* ]]; then
    resolved="$ROOT_DIR/$target"
  elif [[ "$target" == /* ]]; then
    resolved="$ROOT_DIR$target"
  else
    resolved="$(dirname "$source")/$target"
  fi

  if [[ ! -e "$resolved" ]]; then
    echo "ERROR: $source:$line links to missing path: $target" >&2
    status=1
  fi
done < <(
  find . \
    -path './.git' -prune -o \
    -path './output' -prune -o \
    -type f -name '*.md' -print0 \
  | xargs -0 perl -ne '
      while (/!?(?:\[[^]]*\])\(([^)[:space:]]+)(?:[[:space:]]+"[^"]*")?\)/g) {
        print "$ARGV\t$.\t$1\n";
      }
      if (/^[[:space:]]*\[[^]]+\]:[[:space:]]*(?:<([^>]+)>|([^[:space:]]+))/) {
        my $target = defined $1 ? $1 : $2;
        print "$ARGV\t$.\t$target\n";
      }
      while (/`(docs\/[^`[:space:]]+\.md(?:#[^`]*)?)`/g) {
        print "$ARGV\t$.\t$1\n";
      }
      close ARGV if eof;
    '
)

find docs -type f -name '*.md' ! -path 'docs/README.md' | sort > "$inventory_file"
perl -ne '
  while (/!?(?:\[[^]]*\])\(([^)[:space:]#]+\.md)(?:#[^)]*)?\)/g) {
    print "docs/$1\n";
  }
' docs/README.md | sort -u > "$index_file"

if ! diff -u "$inventory_file" "$index_file"; then
  echo "ERROR: docs/README.md must link every documentation page" >&2
  status=1
fi

if (( status != 0 )); then
  echo "[docs] FAILED" >&2
  exit 1
fi

echo "[docs] OK"
