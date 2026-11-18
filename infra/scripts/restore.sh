#!/usr/bin/env bash
# Restore database Saybu dari hasil backup.sh.
#
# PERINGATAN: skema public setiap database akan DIHAPUS lalu diisi ulang dari
# dump. Semua data yang ada akan tertimpa.
#
# Pemakaian:
#   bash infra/scripts/restore.sh <direktori-backup>
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if [[ -f "$ROOT/.env" ]]; then
  set -a
  # shellcheck disable=SC1091
  source "$ROOT/.env"
  set +a
fi

CONTAINER="${POSTGRES_CONTAINER:-saybu-postgres}"
DB_USER="${POSTGRES_USER:-saybu}"

DBS=()
for var in AUTH_DB_DSN CATALOG_DB_DSN SHIPPING_DB_DSN ORDER_DB_DSN PAYMENT_DB_DSN; do
  dsn="${!var:-}"
  [[ -n "$dsn" ]] || continue
  db="${dsn%%\?*}"
  db="${db##*/}"
  [[ -n "$db" ]] && DBS+=("$db")
done
if [[ ${#DBS[@]} -eq 0 ]]; then
  DBS=(saybu_auth saybu_catalog saybu_shipping saybu_orders saybu_payments)
fi

dir="${1:-}"
if [[ -z "$dir" || ! -d "$dir" ]]; then
  echo "pakai: bash infra/scripts/restore.sh <direktori-backup>" >&2
  exit 1
fi

if ! docker inspect "$CONTAINER" >/dev/null 2>&1; then
  echo "!! container '$CONTAINER' tidak ditemukan; jalankan 'make up' dulu" >&2
  exit 1
fi

for db in "${DBS[@]}"; do
  file="$dir/$db.sql"
  if [[ ! -f "$file" ]]; then
    echo "!! lewati $db: $file tidak ada" >&2
    continue
  fi
  echo "== restore $db =="
  docker exec -i "$CONTAINER" psql -q -v ON_ERROR_STOP=1 -U "$DB_USER" -d "$db" \
    -c "DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public;"
  docker exec -i "$CONTAINER" psql -q -v ON_ERROR_STOP=1 -U "$DB_USER" -d "$db" < "$file" > /dev/null
done

echo
echo "Restore selesai dari: $dir"
