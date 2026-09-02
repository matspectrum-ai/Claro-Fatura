#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo "Execute como root." >&2
  exit 1
fi

BINARY="${1:-}"
RELEASE_ID="${2:-$(date -u +%Y%m%dT%H%M%SZ)}"
if [[ -z "$BINARY" || ! -f "$BINARY" ]]; then
  echo "Uso: sudo bash deploy/release.sh <caminho-binario> [release-id]" >&2
  exit 1
fi
if [[ ! "$RELEASE_ID" =~ ^[A-Za-z0-9._-]+$ ]] || [[ "$RELEASE_ID" == .* ]] || [[ "$RELEASE_ID" == *..* ]]; then
  echo "Release ID inválido: $RELEASE_ID" >&2
  exit 1
fi
if [[ ! -f /etc/claro-fatura/claro-fatura.env ]]; then
  echo "Arquivo /etc/claro-fatura/claro-fatura.env ausente." >&2
  exit 1
fi
for cmd in install readlink ln systemctl curl seq sed; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "Comando obrigatório ausente: $cmd" >&2; exit 1; }
done

BASE=/opt/claro-fatura
RELEASE="$BASE/releases/$RELEASE_ID"
CURRENT="$BASE/current"
PREVIOUS="$BASE/previous"

if [[ -e "$RELEASE" ]]; then
  echo "Release já existe: $RELEASE" >&2
  exit 1
fi

install -d -m 0755 "$RELEASE"
install -m 0755 "$BINARY" "$RELEASE/claro-fatura"
chown -R root:root "$RELEASE"

old=""
if [[ -L "$CURRENT" ]]; then
  old="$(readlink -f "$CURRENT")"
  ln -sfn "$old" "$PREVIOUS"
fi

ln -sfn "$RELEASE" "$CURRENT"
systemctl daemon-reload
systemctl restart claro-fatura.service

healthy=0
for _ in $(seq 1 30); do
  if curl --fail --silent --max-time 2 http://127.0.0.1:8080/healthz >/dev/null; then
    healthy=1
    break
  fi
  sleep 1
done

if [[ "$healthy" -ne 1 ]]; then
  echo "Health check falhou para $RELEASE_ID." >&2
  if [[ -n "$old" ]]; then
    echo "Restaurando release anterior: $old" >&2
    ln -sfn "$old" "$CURRENT"
    systemctl restart claro-fatura.service
  else
    systemctl stop claro-fatura.service || true
  fi
  exit 1
fi

systemctl --no-pager --full status claro-fatura.service | sed -n '1,12p' || true
echo "Release ativa: $RELEASE"
