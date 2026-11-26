#!/usr/bin/env bash
# Migrasi goose untuk image sekali jalan di produksi.
# Pemakaian:
#   migrate.sh [up|down]
#
# DSN dibaca dari environment (SERVICE_DB_DSN). Folder migrasi harus berada di
# $MIGRATIONS_ROOT/<service>/migrations (sudah disiapkan Dockerfile.migrate).
set -euo pipefail

direction="${1:-up}"
root="${MIGRATIONS_ROOT:-/srv/backend/services}"

for svc in auth catalog shipping order payment notification; do
  var="$(printf '%s_DB_DSN' "${svc^^}")"
  dsn="${!var:-}"
  dir="$root/$svc/migrations"

  if [[ ! -d "$dir" ]]; then
    echo "!! lewati $svc: folder $dir tidak ada"
    continue
  fi
  if [[ -z "$dsn" ]]; then
    echo "!! lewati $svc: $var kosong"
    continue
  fi

  echo "== goose $direction: $svc =="
  goose -dir "$dir" postgres "$dsn" "$direction"
done

echo "Migrasi $direction selesai."
