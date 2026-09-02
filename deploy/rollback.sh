#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo "Execute como root." >&2
  exit 1
fi

BASE=/opt/claro-fatura
CURRENT="$BASE/current"
PREVIOUS="$BASE/previous"

for cmd in readlink ln mv systemctl curl seq; do
  command -v "$cmd" >/dev/null 2>&1 || { echo "Comando obrigatório ausente: $cmd" >&2; exit 1; }
done

if [[ ! -L "$CURRENT" || ! -L "$PREVIOUS" ]]; then
  echo "Não há release anterior registrada para rollback." >&2
  exit 1
fi

atomic_link() {
  local target="$1" link="$2" tmp="${2}.tmp.$$"
  rm -f "$tmp"
  ln -s "$target" "$tmp"
  mv -Tf "$tmp" "$link"
}

current_target="$(readlink -f "$CURRENT")"
previous_target="$(readlink -f "$PREVIOUS")"

atomic_link "$previous_target" "$CURRENT"
atomic_link "$current_target" "$PREVIOUS"
systemctl restart claro-fatura.service

for _ in $(seq 1 30); do
  if curl --fail --silent --max-time 2 http://127.0.0.1:8080/healthz >/dev/null; then
    echo "Rollback concluído: $previous_target"
    exit 0
  fi
  sleep 1
done

echo "Rollback ativou a release anterior, mas o health check não ficou saudável." >&2
exit 1
