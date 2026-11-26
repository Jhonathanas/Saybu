#!/usr/bin/env bash
# Evaluasi cepat calon VPS sebelum deploy Saybu.
# Jalankan di VPS sebagai user yang akan menjalankan Docker:
#   bash infra/scripts/check-vps.sh
#
# Exit code: 0 bila tidak ada FAIL, 1 bila ada syarat minimum yang tidak terpenuhi.
set -uo pipefail

pass=0
warn=0
fail=0

ok()   { printf '[PASS] %s\n' "$1"; ((pass++)); }
note() { printf '[WARN] %s\n' "$1"; ((warn++)); }
bad()  { printf '[FAIL] %s\n' "$1"; ((fail++)); }
info() { printf '       %s\n' "$1"; }

printf '=== Evaluasi VPS Saybu ===\n'
printf 'Waktu   : %s\n' "$(date -Iseconds)"
printf 'Host    : %s\n' "$(hostname)"

# --- Sistem ---
os="$(. /etc/os-release 2>/dev/null && printf '%s' "${PRETTY_NAME:-tidak-diketahui}")"
arch="$(uname -m)"
info "OS      : $os"
info "Arsitek : $arch"
case "$arch" in
  x86_64|aarch64|arm64) ok "Arsitek didukung image Docker" ;;
  *) note "Arsitek $arch belum diverifikasi; CD saat ini mem-build linux/amd64" ;;
esac

# --- CPU / RAM / disk ---
cores="$(nproc 2>/dev/null || printf '0')"
if (( cores >= 2 )); then ok "CPU $cores core (minimal 2)"
else bad "CPU hanya $cores core (minimal 2)"; fi

ram_mb="$(awk '/MemTotal/ {printf "%d", $2/1024}' /proc/meminfo 2>/dev/null || printf '0')"
if (( ram_mb >= 4096 )); then ok "RAM ${ram_mb} MB (target ≥ 4096 MB)"
elif (( ram_mb >= 3072 )); then note "RAM ${ram_mb} MB; Kafka bisa berat, disarankan ≥ 4096 MB"
else bad "RAM ${ram_mb} MB di bawah minimal 4096 MB"; fi

disk_gb="$(df -P / 2>/dev/null | awk 'NR==2 {printf "%d", $4/1024/1024}')"
if (( disk_gb >= 40 )); then ok "Disk kosong ${disk_gb} GB"
elif (( disk_gb >= 20 )); then note "Disk kosong ${disk_gb} GB; backup + Kafka bisa penuh"
else bad "Disk kosong ${disk_gb} GB (disarankan ≥ 40 GB)"; fi

swap_mb="$(awk '/SwapTotal/ {printf "%d", $2/1024}' /proc/meminfo 2>/dev/null || printf '0')"
if (( swap_mb >= 1024 )); then ok "Swap ${swap_mb} MB"
else note "Swap ${swap_mb} MB; tambahkan swap bila RAM pas-pasan"; fi

# --- Docker ---
if command -v docker >/dev/null 2>&1; then
  docker_ver="$(docker version --format '{{.Server.Version}}' 2>/dev/null || printf '')"
  if [[ -n "$docker_ver" ]]; then ok "Docker daemon aktif (v$docker_ver)"
  else bad "Docker terpasang tetapi daemon tidak bisa diakses"; fi

  if docker compose version >/dev/null 2>&1; then
    ok "Compose v2: $(docker compose version --short 2>/dev/null)"
  else
    bad "docker compose v2 tidak tersedia"
  fi

  if [[ "$(id -u)" == "0" ]] || id -nG | grep -qw docker; then
    ok "User bisa memanggil docker tanpa sudo"
  else
    bad "User belum masuk grup docker (atau jalankan sebagai root)"
  fi
else
  bad "Docker belum terpasang"
fi

# --- Firewall & port ---
if command -v ufw >/dev/null 2>&1; then
  info "UFW     : $(ufw status 2>/dev/null | head -1)"
else
  note "UFW tidak ditemukan; pastikan firewall provider hanya membuka 22/80/443"
fi

if command -v ss >/dev/null 2>&1; then
  used="$(ss -tln 2>/dev/null | awk 'NR>1 {print $4}' | grep -E ':(80|443)$' | tr '\n' ' ')"
  if [[ -z "$used" ]]; then ok "Port 80/443 belum dipakai proses lain"
  else bad "Port 80/443 sudah dipakai: $used"; fi
else
  note "ss tidak tersedia; cek manual port 80/443"
fi

if command -v docker >/dev/null 2>&1 && docker info >/dev/null 2>&1; then
  bentrok="$(docker ps --format '{{.Ports}}' 2>/dev/null | grep -E '(0\.0\.0\.0|:::)(80|443|5432|9092)->' || true)"
  if [[ -z "$bentrok" ]]; then ok "Tidak ada container yang memakai port 80/443/5432/9092"
  else bad "Port dipakai container lain: $bentrok"; fi
fi

# --- Jaringan keluar ---
if command -v curl >/dev/null 2>&1; then
  # GHCR mengembalikan 401 untuk anonymous /v2/ — itu tanda host bisa dijangkau.
  code="$(curl -sS --max-time 8 -o /dev/null -w '%{http_code}' https://ghcr.io/v2/ 2>/dev/null || printf '000')"
  if [[ "$code" == "200" || "$code" == "401" ]]; then
    ok "Akses keluar ke GHCR lancar (HTTP $code)"
  else
    bad "Gagal mengakses https://ghcr.io/v2/ (HTTP $code; CD tidak bisa pull image)"
  fi
  if curl -fsS --max-time 8 -o /dev/null https://acme-v02.api.letsencrypt.org/directory 2>/dev/null; then
    ok "Akses ke Let's Encrypt lancar (sertifikat Caddy)"
  else
    note "Gagal mengakses Let's Encrypt; Caddy tidak bisa dapat sertifikat otomatis"
  fi
else
  note "curl tidak terpasang; tidak bisa menguji akses keluar"
fi

# --- Zona waktu ---
tz="$(timedatectl show -p Timezone --value 2>/dev/null || date +%Z)"
if [[ "$tz" == "Asia/Jakarta" ]]; then ok "Timezone VPS $tz (cron backup 02:00 WIB)"
else note "Timezone VPS $tz; set Asia/Jakarta agar jadwal backup sesuai"; fi

printf '\nRingkasan: %d PASS, %d WARN, %d FAIL\n' "$pass" "$warn" "$fail"
if (( fail > 0 )); then
  printf 'VPS belum siap. Perbaiki item FAIL lalu jalankan ulang.\n' >&2
  exit 1
fi
printf 'VPS layak untuk deploy. Lanjutkan ke docs/DEPLOY_CHECKLIST.md.\n'
