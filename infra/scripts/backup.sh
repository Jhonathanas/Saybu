#!/usr/bin/env bash
# Backup semua database Saybu ke file SQL.
#
# pg_dump hanya tersedia di dalam container Postgres (host tidak perlu memasang
# klien postgres). Hasil ditulis ke <dest>/<timestamp>/<db>.sql (plain SQL).
# Nama database diambil dari DSN di .env.
#
# Pemakaian:
#   bash infra/scripts/backup.sh [direktori-tujuan]
# Default direktori: <repo>/backups
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

# Kumpulkan nama database dari DSN tiap service (buang query string & host).
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

dest="${1:-$ROOT/backups}"
stamp="$(date +%Y%m%d-%H%M%S)"
out="$dest/$stamp"
mkdir -p "$out"

if ! docker inspect "$CONTAINER" >/dev/null 2>&1; then
  echo "!! container '$CONTAINER' tidak ditemukan; jalankan 'make up' dulu" >&2
  exit 1
fi

for db in "${DBS[@]}"; do
  echo "== backup $db =="
  docker exec "$CONTAINER" pg_dump -U "$DB_USER" --no-owner --no-privileges "$db" > "$out/$db.sql"
  echo "   -> $out/$db.sql ($(wc -c < "$out/$db.sql") byte)"
done

{
  echo "created_at=$(date -Iseconds)"
  echo "container=$CONTAINER"
  echo "databases=${DBS[*]}"
} > "$out/manifest.txt"

echo
echo "Backup selesai: $out"
