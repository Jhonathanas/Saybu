#!/usr/bin/env bash
# Menjalankan migrasi goose untuk semua service yang punya folder migrations.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if [[ -f "$ROOT/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/.env"
  set +a
fi

declare -A dsn=(
  [auth]="${AUTH_DB_DSN:-}"
  [catalog]="${CATALOG_DB_DSN:-}"
  [shipping]="${SHIPPING_DB_DSN:-}"
  [order]="${ORDER_DB_DSN:-}"
  [payment]="${PAYMENT_DB_DSN:-}"
  [notification]="${NOTIFICATION_DB_DSN:-}"
)

direction="${1:-up}"

for svc in auth catalog shipping order payment notification; do
  dir="$ROOT/backend/services/$svc/migrations"
  [[ -d "$dir" ]] || continue
  if [[ -z "${dsn[$svc]}" ]]; then
    echo "!! DSN untuk $svc kosong, lewati"
    continue
  fi
  echo "== goose $direction: $svc =="
  goose -dir "$dir" postgres "${dsn[$svc]}" "$direction"
done
