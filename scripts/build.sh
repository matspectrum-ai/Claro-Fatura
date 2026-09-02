#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"
bash scripts/fetch-sheetjs.sh
bash scripts/fetch-qrcode.sh
CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "${1:-claro-fatura}" ./cmd/server
