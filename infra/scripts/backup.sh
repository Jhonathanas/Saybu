#!/usr/bin/env bash
# Backup semua database Saybu ke file SQL terkompresi.
#
# pg_dump hanya tersedia di dalam container Postgres (host tidak perlu memasang
# klien postgres). Hasil ditulis ke <dest>/<timestamp>/<db>.sql.gz.
# Nama database diambil dari DSN di .env.
#
# Pemakaian:
#   bash infra/scripts/backup.sh [direktori-tujuan]
# Default direktori: <repo>/backups
#
# Environment opsional:
#   BACKUP_COMPRESS=false     # simpan .sql tanpa gzip
#   BACKUP_KEEP_DAYS=7        # hapus snapshot lokal lebih tua dari N hari (0 = mati)
#   BACKUP_REMOTE=name:path   # rclone remote; bila diisi, snapshot juga di-upload
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
COMPRESS="${BACKUP_COMPRESS:-true}"
KEEP_DAYS="${BACKUP_KEEP_DAYS:-7}"
REMOTE="${BACKUP_REMOTE:-}"

# Kumpulkan nama database dari DSN tiap service (buang query string & host).
DBS=()
for var in AUTH_DB_DSN CATALOG_DB_DSN SHIPPING_DB_DSN ORDER_DB_DSN PAYMENT_DB_DSN NOTIFICATION_DB_DSN; do
  dsn="${!var:-}"
  [[ -n "$dsn" ]] || continue
  db="${dsn%%\?*}"
  db="${db##*/}"
  [[ -n "$db" ]] && DBS+=("$db")
done
if [[ ${#DBS[@]} -eq 0 ]]; then
  DBS=(saybu_auth saybu_catalog saybu_shipping saybu_orders saybu_payments saybu_notifications)
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
  if [[ "$COMPRESS" == "true" ]]; then
    file="$out/$db.sql.gz"
    docker exec "$CONTAINER" pg_dump -U "$DB_USER" --no-owner --no-privileges "$db" | gzip -c > "$file"
  else
    file="$out/$db.sql"
    docker exec "$CONTAINER" pg_dump -U "$DB_USER" --no-owner --no-privileges "$db" > "$file"
  fi
  echo "   -> $file ($(wc -c < "$file") byte)"
done

{
  echo "created_at=$(date -Iseconds)"
  echo "container=$CONTAINER"
  echo "databases=${DBS[*]}"
  echo "compression=${COMPRESS}"
  echo "retention_days=${KEEP_DAYS}"
  echo "remote=${REMOTE:-tidak-ada}"
} > "$out/manifest.txt"

if [[ -n "$REMOTE" ]]; then
  if ! command -v rclone >/dev/null 2>&1; then
    echo "!! BACKUP_REMOTE diisi tetapi rclone tidak ditemukan; upload dilewati" >&2
  else
    echo "== upload ke $REMOTE/$stamp =="
    rclone copy "$out" "$REMOTE/$stamp"
  fi
fi

if [[ "$KEEP_DAYS" =~ ^[0-9]+$ ]] && (( KEEP_DAYS > 0 )); then
  echo "== hapus snapshot lokal lebih tua dari $KEEP_DAYS hari =="
  find "$dest" -mindepth 1 -maxdepth 1 -type d -mtime "+$KEEP_DAYS" -print -exec rm -rf {} +
fi

echo
echo "Backup selesai: $out"
