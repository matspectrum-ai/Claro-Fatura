#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEST="$ROOT/internal/httpapi/web/assets/vendor/xlsx.full.min.js"
URL="https://raw.githubusercontent.com/SheetJS/sheetjs/v0.18.5/dist/xlsx.full.min.js"
EXPECTED_BLOB="16e013fceefc689cabc5be352099199847a0e67f"

mkdir -p "$(dirname "$DEST")"
if [[ -f "$DEST" ]] && [[ "$(git hash-object "$DEST")" == "$EXPECTED_BLOB" ]]; then
  exit 0
fi

tmp="${DEST}.tmp"
trap 'rm -f "$tmp"' EXIT
curl --fail --silent --show-error --location --retry 3 --retry-delay 1 "$URL" -o "$tmp"
actual="$(git hash-object "$tmp")"
if [[ "$actual" != "$EXPECTED_BLOB" ]]; then
  echo "SheetJS integrity check failed: expected $EXPECTED_BLOB, got $actual" >&2
  exit 1
fi
mv "$tmp" "$DEST"
trap - EXIT
