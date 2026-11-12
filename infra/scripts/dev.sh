#!/usr/bin/env bash
# Menjalankan semua service Go secara paralel untuk development.
# Ctrl+C akan menghentikan semuanya.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT/backend"

services=(gateway auth catalog shipping order payment notification)
pids=()

cleanup() {
  trap - EXIT INT TERM
  if ((${#pids[@]})); then
    kill "${pids[@]}" 2>/dev/null || true
    wait 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

for svc in "${services[@]}"; do
  echo "starting $svc"
  go run "./services/$svc/cmd" &
  pids+=($!)
done

wait
