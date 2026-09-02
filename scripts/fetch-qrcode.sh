#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEST="$ROOT/internal/httpapi/web/assets/vendor/qrcode.min.js"
URL="https://cdnjs.cloudflare.com/ajax/libs/qrcode-generator/1.4.4/qrcode.min.js"
EXPECTED_SHA512="64348f31afc93350feee4760db1dc1b2bb90e93fc9a49a378d60d690266c3fee72572a7529f112a8b23eed1a81e64db817ba76715ee2d981e1ffcf74cba3fa4b"

mkdir -p "$(dirname "$DEST")"
if [[ -f "$DEST" ]] && [[ "$(sha512sum "$DEST" | awk '{print $1}')" == "$EXPECTED_SHA512" ]]; then
  exit 0
fi

tmp="${DEST}.tmp"
trap 'rm -f "$tmp"' EXIT
curl --fail --silent --show-error --location --retry 3 --retry-delay 1 "$URL" -o "$tmp"
actual="$(sha512sum "$tmp" | awk '{print $1}')"
if [[ "$actual" != "$EXPECTED_SHA512" ]]; then
  echo "QR encoder integrity check failed: expected $EXPECTED_SHA512, got $actual" >&2
  exit 1
fi
mv "$tmp" "$DEST"
trap - EXIT
