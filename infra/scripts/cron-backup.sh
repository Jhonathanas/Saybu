#!/usr/bin/env bash
# Wrapper cron untuk backup harian. Log ditulis ke infra/logs/backup.log.
# Contoh crontab (lihat infra/cron/saybu-backup.cron):
#   0 2 * * * /opt/saybu/infra/scripts/cron-backup.sh
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
LOG_DIR="$ROOT/infra/logs"
mkdir -p "$LOG_DIR" "$ROOT/backups"

{
  echo "--- $(date -Iseconds) ---"
  bash "$ROOT/infra/scripts/backup.sh" "$ROOT/backups"
} >> "$LOG_DIR/backup.log" 2>&1
