# DEPLOY CHECKLIST — Saybu

Dokumen ini merangkum **yang masih harus diputuskan/disiapkan di luar repo** sebelum M7
selesai. Artefak teknis (Compose, Caddy, Dockerfile, CD, backup) sudah ada dan ter-commit;
prosedur operasionalnya ada di `docs/RUNBOOK.md` §17.

Terakhir diperbarui: 2026-11-26

---

## Status input saat ini

| Item | Status | Keterangan |
|---|---|---|
| VPS ≥ 4 GB + Docker | Perlu evaluasi | Jalankan `bash infra/scripts/check-vps.sh` di calon VPS |
| Repo GitHub/GHCR | Sudah ada | **Dibutuhkan:** URL repo & owner GHCR (huruf kecil) |
| Domain + DNS | Belum ada | Perlu nama domain, A/AAAA record, email admin |
| Data produk & foto | Siap | Perlu dimasukkan ke produksi lewat panel admin/seed |
| Telegram bot | Belum ada | Token + chat id untuk notifikasi & alert Grafana |
| Uji order nyata | Belum | Menunggu deploy + pembayaran nyata |

---

## 1. Evaluasi VPS

Jalankan di VPS sebagai user yang akan menjalankan Docker:

```bash
git clone <url-repo> /opt/saybu     # bila belum ada
cd /opt/saybu
bash infra/scripts/check-vps.sh
```

Kirim ringkasan `PASS/WARN/FAIL` beserta OS, CPU, RAM, disk, dan hasil akses GHCR.
Syarat minimum:

- Ubuntu LTS, arsitek `x86_64`, **≥ 2 vCPU**, **≥ 4 GB RAM**, **≥ 40 GB disk kosong**
- Docker daemon + Compose v2 aktif, user masuk grup `docker`
- Firewall penyedia/VPS hanya membuka **22, 80, 443**
- Port 80/443 belum dipakai nginx/Caddy lain
- Akses keluar ke `ghcr.io` dan `acme-v02.api.letsencrypt.org` lancar
- Timezone `Asia/Jakarta` (cron backup 02:00 WIB)

Script mengembalikan exit code 1 bila ada item `FAIL`.

---

## 2. GitHub / GHCR

Berikan informasi berikut agar workflow `.github/workflows/cd.yml` bisa dipakai:

- URL repo, mis. `https://github.com/<owner>/saybu`
- Owner GHCR = `<owner>` dalam huruf kecil
- Apakah package GHCR privat (default: privat; token `GITHUB_TOKEN` cukup untuk repo yang sama)

Setelah itu di lokal:

```bash
git remote add origin https://github.com/<owner>/saybu.git
git push -u origin main
```

Secrets GitHub (Settings → Environments → `production` → Secrets):

| Secret | Nilai |
|---|---|
| `SSH_HOST` | IP/public host VPS |
| `SSH_USER` | user SSH (mis. `saybu`) |
| `SSH_KEY` | private key (tanpa passphrase untuk otomasi) |
| `SSH_PORT` | opsional, default `22` |
| `DEPLOY_PATH` | mis. `/opt/saybu` |

`GHCR_OWNER` di `.env` VPS diset ke owner huruf kecil; `IMAGE_TAG` diisi SHA commit oleh CD.

---

## 3. Domain & DNS

Pilih domain produksi, lalu di pengelola DNS:

| Record | Nama | Nilai | TTL |
|---|---|---|---|
| A | `@` | IP publik VPS | 300 |
| A | `www` | IP publik VPS | 300 |
| AAAA | `@` / `www` | alamat IPv6 VPS (bila ada) | 300 |
| MX | `@` | sesuai email provider | default |

Lanjutkan hanya setelah `dig +short <domain>` mengembalikan IP VPS. Isi `.env` VPS:

```bash
SITE_DOMAIN=contoh.com          # tanpa https://
ADMIN_EMAIL=admin@contoh.com    # untuk ACME Let's Encrypt
```

Caddy akan mendapat sertifikat otomatis saat pertama kali `up`. **Tanpa domain,
HTTPS tidak bisa diaktifkan** — jangan pakai IP untuk produksi.

---

## 4. Telegram

1. Buat bot via [@BotFather](https://t.me/BotFather) → `/newbot`, simpan token.
2. Dapatkan chat id admin (mis. via [@userinfobot](https://t.me/userinfobot)).
3. Isi di `.env` VPS:

```bash
TELEGRAM_BOT_TOKEN=123456:ABC...
TELEGRAM_ADMIN_CHAT_ID=987654321
```

4. Setelah deploy, restart konsumer & Grafana agar memakai nilai baru:

```bash
docker compose -f docker-compose.prod.yml up -d --force-recreate notification
docker compose -f docker-compose.prod.yml --profile observability up -d --force-recreate grafana
```

Kirim satu pesan uji dari halaman notifikasi/alert lalu cek penerimaannya.

---

## 5. Data produk & foto

Data katalog sudah disiapkan. Sebelum go-live:

- [ ] Import/seed kategori, produk, harga, stok, dan zona ongkir ke DB produksi
- [ ] Pastikan foto produk reachable di `https://<domain>/images/...` atau `/uploads/...`
- [ ] Cek harga masih `int64` rupiah, stok satuan terkecil, dan snapshot alamat di order
- [ ] Verifikasi akun admin & rotasi `JWT_SECRET`, `POSTGRES_PASSWORD`, `GRAFANA_PASSWORD`

> Gunakan panel admin `/admin/produk` untuk data final bila seed contoh tidak dipakai.

---

## 6. Deploy & verifikasi

Urutan lengkap ada di `docs/RUNBOOK.md` §17. Ringkasnya:

```bash
cd /opt/saybu
set -a; . ./.env; set +a
make prod-config
make prod-migrate
make prod-up
curl -sS https://<domain>/api/v1/version
curl -sS -o /dev/null -w '%{http_code}\n' https://<domain>/healthz
```

---

## 7. Go-live checklist

- [ ] HTTPS aktif & redirect HTTP → HTTPS
- [ ] Beranda, katalog, detail produk, keranjang, checkout berfungsi di mobile & desktop
- [ ] Order nyata dibuat dengan harga/ongkir dihitung server (bukan dari browser)
- [ ] Bukti transfer diunggah admin & status berubah sampai `PAID`
- [ ] COD lunas otomatis saat status `COMPLETED`
- [ ] Notifikasi Telegram diterima (order baru, stok menipis, alert Grafana)
- [ ] Backup harian pertama tampil di `backups/<timestamp>/manifest.txt`
- [ ] Rollback diuji dengan mengembalikan `IMAGE_TAG` sebelumnya (tanpa restore bila skema sama)
- [ ] `M5-12` uji usability dengan orangtua dijadwalkan (task terakhir M5)
