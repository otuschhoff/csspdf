#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

BASE="examples/layered/styles/corporate/base.css"
DOC="examples/layered/styles/document/doc.css"
OVR="examples/layered/styles/overrides/customer.css"

for f in "$BASE" "$DOC" "$OVR"; do
  if [[ ! -f "$f" ]]; then
    echo "missing style file: $f" >&2
    exit 1
  fi
done

normalize() {
  sed 's:/\*.*\*/::g' "$1" \
    | tr 'A-Z' 'a-z' \
    | sed 's/[[:space:]]\+/ /g' \
    | sed 's/^ //;s/ $//' \
    | grep -v '^$'
}

base_tmp=$(mktemp)
doc_tmp=$(mktemp)
ovr_tmp=$(mktemp)
trap 'rm -f "$base_tmp" "$doc_tmp" "$ovr_tmp"' EXIT

normalize "$BASE" > "$base_tmp"
normalize "$DOC" > "$doc_tmp"
normalize "$OVR" > "$ovr_tmp"

dup_base_doc=$(comm -12 <(sort "$base_tmp") <(sort "$doc_tmp") | wc -l | tr -d ' ')
dup_doc_ovr=$(comm -12 <(sort "$doc_tmp") <(sort "$ovr_tmp") | wc -l | tr -d ' ')
dup_base_ovr=$(comm -12 <(sort "$base_tmp") <(sort "$ovr_tmp") | wc -l | tr -d ' ')

echo "Layered CSS duplication metrics"
echo "- base vs document duplicate normalized lines: $dup_base_doc"
echo "- document vs override duplicate normalized lines: $dup_doc_ovr"
echo "- base vs override duplicate normalized lines: $dup_base_ovr"
