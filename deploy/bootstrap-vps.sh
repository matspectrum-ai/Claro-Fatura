#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo "Execute como root." >&2
  exit 1
fi

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOMAIN="${1:-}"
if [[ -z "$DOMAIN" ]]; then
  echo "Uso: sudo bash deploy/bootstrap-vps.sh <dominio>" >&2
  exit 1
fi

if ! id claro-fatura >/dev/null 2>&1; then
  useradd --system --home /nonexistent --shell /usr/sbin/nologin claro-fatura
fi

install -d -m 0755 /opt/claro-fatura/releases
install -d -m 0750 -o root -g claro-fatura /etc/claro-fatura

if [[ ! -f /etc/claro-fatura/claro-fatura.env ]]; then
  install -m 0640 -o root -g claro-fatura "$ROOT/deploy/claro-fatura.env.example" /etc/claro-fatura/claro-fatura.env
  echo "Criado /etc/claro-fatura/claro-fatura.env. Preencha as credenciais antes do primeiro deploy."
fi

install -m 0644 "$ROOT/deploy/systemd/claro-fatura.service" /etc/systemd/system/claro-fatura.service

nginx_target=""
if [[ -d /etc/nginx/sites-available ]]; then
  nginx_target=/etc/nginx/sites-available/claro-fatura.conf
  sed "s/__DOMAIN__/$DOMAIN/g" "$ROOT/deploy/nginx/claro-fatura.conf" > "$nginx_target"
  ln -sfn "$nginx_target" /etc/nginx/sites-enabled/claro-fatura.conf
  rm -f /etc/nginx/sites-enabled/default
elif [[ -d /etc/nginx/conf.d ]]; then
  nginx_target=/etc/nginx/conf.d/claro-fatura.conf
  sed "s/__DOMAIN__/$DOMAIN/g" "$ROOT/deploy/nginx/claro-fatura.conf" > "$nginx_target"
else
  echo "Diretório de configuração do Nginx não encontrado." >&2
  exit 1
fi

nginx -t
systemctl daemon-reload
systemctl enable claro-fatura.service
systemctl reload nginx

echo "Bootstrap concluído. Edite /etc/claro-fatura/claro-fatura.env e use deploy/release.sh para publicar o binário."
