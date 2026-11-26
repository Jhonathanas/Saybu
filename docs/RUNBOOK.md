# RUNBOOK — Saybu

| Field | Value |
|---|---|
| Versi | 1.0 |
| Terakhir diperbarui | 2026-11-18 |
| Cakupan | M6-08 (dokumentasi operasional) |

Panduan operasional & penanganan insiden untuk Saybu. Setiap insiden berisi
**gejala → diagnosis → penanganan → verifikasi**. Perintah dijalankan dari akar
repo (`/path/ke/saybu`) kecuali disebut lain.

---

## 1. Peta cepat

| Komponen | Alamat / perintah |
|---|---|
| Gateway REST | `http://localhost:8081` |
| Health/ready/metrics | gateway `:8081`, admin `:9101`–`:9105`, notification `:8086` |
| Kafka UI | `http://localhost:8090` |
| Prometheus | `http://localhost:9090` (`make observability`) |
| Grafana | `http://localhost:3000` (`admin`/`admin`) |
| Postgres | `docker exec -it saybu-postgres psql -U saybu -d saybu_orders` |
| Log service (dev) | stdout `make dev`, atau file di `LOG_DIR` bila dijalankan via `bin/` |

Perintah rutin:

```bash
make ps                 # status container
make logs               # log container (follow)
curl -s localhost:8081/healthz   # liveness
curl -s localhost:8081/readyz    # readiness (DB + downstream)
curl -s localhost:8081/metrics | grep saybu_
```

Topik Kafka: `order.events.v1`, `payment.events.v1`, `catalog.events.v1`,
`notification.commands.v1` (+ masing-masing `.dlq`).
Consumer group: `order-svc`, `catalog-svc`, `payment-svc`, `notification-svc`.

---

## 2. Checklist triase umum

1. Cek container: `make ps` → pastikan `saybu-postgres`, `saybu-kafka` **healthy**.
2. Cek readiness gateway: `curl -s localhost:8081/readyz`.
3. Cek Prometheus targets: `http://localhost:9090/targets` → semua `up`.
4. Cek dashboard Grafana "Saybu — Observability" (rate, error, latency, lag).
5. Baru masuk ke insiden spesifik di bawah.

---

## 3. Insiden: service Go tidak hidup / crash

**Gejala:** `readyz` 503, endpoint tertentu 502/500, log service berhenti.

**Diagnosis**
```bash
make ps
# lihat log service yang bermasalah (dev)
#   contoh bila dijalankan dari bin/: tail -f bin/<svc>.log
```

**Penanganan**
```bash
make build                       # pastikan binary terbaru
make dev                         # jalankan ulang semua service
# atau satu service: cd backend && go run ./services/<svc>/cmd
```

**Verifikasi:** `curl -s localhost:<admin_port>/readyz` → 200; lakukan satu
request nyata (mis. `curl -s localhost:8081/api/v1/products`).

> Catatan: `pkill -f 'go-build.*/exe/cmd'` bisa mematikan shell pemanggil karena
> pola cocok dengan dirinya sendiri — matikan berdasarkan PID eksplisit.

---

## 4. Insiden: Postgres down

**Gejala:** `/readyz` 503 dengan cek `postgres` gagal; semua endpoint tulis error.

**Diagnosis**
```bash
make ps | grep postgres
docker logs --tail=100 saybu-postgres
docker exec saybu-postgres pg_isready -U saybu
```

**Penanganan**
```bash
make up                 # compose up -d (restart bila mati)
docker restart saybu-postgres
```
Bila data rusak → lihat §9 (restore).

**Verifikasi:** `docker exec saybu-postgres pg_isready -U saybu` → `accepting
connections`; `curl -s localhost:9104/readyz` → 200.

---

## 5. Insiden: Kafka down

**Gejala:** `/readyz` 503 dengan cek `kafka` gagal; event tidak mengalir; outbox
menumpuk (lihat §7).

**Diagnosis**
```bash
make ps | grep kafka
docker logs --tail=100 saybu-kafka
docker exec saybu-kafka /opt/kafka/bin/kafka-topics.sh \
  --bootstrap-server localhost:9092 --list
```

**Penanganan**
```bash
docker restart saybu-kafka
# bila broker korup di dev (kehilangan data), reset volume:
#   docker compose down kafka && docker volume rm saybu_kafkadata && make up
```
> Reset volume Kafka menghapus semua pesan. Outbox di tiap service akan mengirim
> ulang event yang belum terkirim; consumer memakai `ConsumeResetOffset(AtStart)`
> sehingga grup baru akan memutar ulang riwayat.

**Verifikasi:** `kafka-topics.sh --list` menampilkan 4 topik + `.dlq`;
`curl -s localhost:8081/readyz` → 200.

---

## 6. Insiden: consumer lag tinggi

**Gejala:** panel "Kafka consumer lag" naik terus; notifikasi/efek samping terlambat.

**Diagnosis**
```bash
docker exec saybu-kafka /opt/kafka/bin/kafka-consumer-groups.sh \
  --bootstrap-server localhost:9092 --describe --all-groups
# atau metrik: saybu_<svc>_kafka_consumer_lag
```

**Penanganan**
- Lag naik sementara saat lonjakan → normal, tunggu.
- Lag terus naik → consumer mati/lambat:
  1. Pastikan consumer service hidup (§3).
  2. Cek error berulang di log (mis. DB lambat, bug handler).
  3. Bila handler selalu gagal, pesan akan masuk `.dlq` setelah 3 percobaan (§8).
  4. Scale: jalankan instance consumer tambahan. Setiap topik memakai 3 partisi
     (`infra/kafka/create-topics.sh`, `KAFKA_NUM_PARTITIONS=3` di compose), jadi hingga
     3 instance per consumer group bisa bekerja paralel; jangan melebihi jumlah partisi.

**Verifikasi:** lag kembali mendekati 0 dan stabil di dashboard.

---

## 7. Insiden: outbox menumpuk (event tidak terkirim)

**Gejala:** metrik `saybu_<svc>_outbox_lag_seconds` naik; event tidak sampai Kafka.

**Diagnosis**
```bash
# ganti DB sesuai service (saybu_orders, saybu_catalog, saybu_payments)
docker exec saybu-postgres psql -U saybu -d saybu_orders -c \
  "select count(*) filter (where published_at is null) as pending,
          max(attempts) as max_attempts
   from outbox;"
docker exec saybu-postgres psql -U saybu -d saybu_orders -c \
  "select id, topic, attempts, last_error from outbox
   where published_at is null order by id limit 10;"
```

**Penanganan**
- `pending` banyak + Kafka sehat → relay berhenti. Pastikan service hidup (§3);
  relay berjalan sebagai goroutine di service.
- `last_error` menunjuk masalah (mis. broker unreachable) → perbaiki Kafka (§5).
- Baris dengan `attempts` besar dan error permanen → periksa payload; perbaiki
  penyebab, lalu relay akan mengirim ulang pada percobaan berikutnya.

**Verifikasi:** `pending` kembali 0; `outbox_lag_seconds` → 0.

---

## 8. Insiden: dead-letter queue (DLQ) terisi

**Gejala:** log `kafka ... -> DLQ`; topik `<topic>.dlq` bertambah.

**Diagnosis**
```bash
docker exec saybu-kafka /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 --topic order.events.v1.dlq \
  --from-beginning --max-messages 10
```
Pesan DLQ membawa header/field asal (`event_id`, error). Cocokkan dengan log
service untuk akar masalah.

**Penanganan**
1. Perbaiki bug/penyebab (mis. skema payload, data hilang).
2. Putar ulang pesan yang valid: kirim kembali ke topik asal (consumer akan
   memproses; handler idempotent memakai `processed_events`).
3. Bila pesan memang tak bisa diproses (data rusak), dokumentasikan & hapus.

**Verifikasi:** pesan yang diperbaiki berhasil diproses; DLQ tidak bertambah lagi.

**Catatan notifikasi:** bila konsumer notification crash setelah mengklaim event tetapi sebelum
mengirim Telegram, baris tetap berstatus `pending` dan duplikat berikutnya dilewati. Bersihkan
klaim yang nyangkut agar event diproses ulang:

```sql
-- periksa
SELECT event_id, event_type, created_at FROM saybu_notifications.processed_events
WHERE status = 'pending' AND created_at < now() - interval '10 minutes';
-- hapus hanya event yang memang ingin diulang
DELETE FROM saybu_notifications.processed_events
WHERE event_id = '<event_id>' AND status = 'pending';
```

---

## 9. Insiden: data rusak / perlu restore

**Gejala:** data hilang/korup; migrasi salah; perlu kembali ke snapshot.

**Diagnosis**
```bash
ls -1 backups/                 # snapshot yang tersedia
cat backups/<timestamp>/manifest.txt   # berisi 6 DB, kompresi, retensi, remote
```

**Penanganan (PERINGATAN: menimpa data)**
```bash
# 1. Buat backup kondisi sekarang dulu (jaring pengaman)
make backup

# 2. Hentikan penulisan (opsional tapi disarankan)
#    matikan service Go, sisakan Postgres

# 3. Restore (menerima .sql.gz maupun .sql)
make restore DIR=backups/<timestamp>
```
Setara dengan: `bash infra/scripts/restore.sh backups/<timestamp>`. Skrip
menghapus skema `public` tiap DB lalu memuat ulang dari `pg_dump` (mencakup tabel
`goose_db_version` sehingga versi migrasi ikut pulih). Enam DB yang dicakup:
`saybu_auth`, `saybu_catalog`, `saybu_shipping`, `saybu_orders`, `saybu_payments`,
`saybu_notifications`. Per produksi, ambil salinan juga dari `BACKUP_REMOTE`
bila snapshot lokal hilang.

**Verifikasi**
```bash
docker exec saybu-postgres psql -U saybu -d saybu_orders -t -c "select count(*) from orders;"
curl -s localhost:8081/api/v1/products -o /dev/null -w '%{http_code}\n'   # 200
```
Jalankan `make up` / `make dev` kembali bila tadi dihentikan.

---

## 10. Insiden: migrasi gagal / salah versi

**Gejala:** service gagal start; `goose` error; tabel tidak sesuai.

**Diagnosis**
```bash
cd backend/services/<svc>
goose -dir migrations postgres "$DSN" status
```

**Penanganan**
```bash
make migrate            # goose up semua DB
make migrate-down       # goose down (satu langkah)
# atau satu service:
cd backend/services/<svc>
goose -dir migrations postgres "$DSN" up
```
Migrasi harus bisa naik & turun (aturan AGENTS.md #9). Bila ragu, restore dulu
(§9) sebelum mencoba ulang.

---

## 11. Insiden: pembayaran/verifikasi macet

**Gejala:** pesanan tetap `WAITING_VERIFICATION` walau admin sudah verifikasi;
status tidak berpindah.

**Diagnosis**
```bash
# cek event payment terkirim
docker exec saybu-kafka /opt/kafka/bin/kafka-console-consumer.sh \
  --bootstrap-server localhost:9092 --topic payment.events.v1 \
  --from-beginning --max-messages 5
# cek outbox payment & order
docker exec saybu-postgres psql -U saybu -d saybu_payments -c \
  "select count(*) from outbox where published_at is null;"
```

**Penanganan**
- Bila event ada tapi order tidak berubah → consumer `order-svc` bermasalah (§3, §6).
- Bila event tidak ada → outbox payment menumpuk (§7).
- Perubahan status manual oleh admin: `PATCH /admin/orders/{id}/status`
  (transisi divalidasi; ilegal → 409).

**Verifikasi:** status pesanan berpindah sesuai alur; notifikasi terkirim (bila
Telegram aktif).

---

## 12. Insiden: notifikasi Telegram tidak terkirim

**Gejala:** event diproses tapi admin tidak menerima pesan.

**Diagnosis**
```bash
# cek konfigurasi
grep TELEGRAM .env
# cek log notification
docker logs --tail=50 saybu-notification 2>/dev/null || true
```
- `TELEGRAM_BOT_TOKEN` / `TELEGRAM_ADMIN_CHAT_ID` kosong → notifikasi **sengaja
  dilewati** (log warning "telegram nonaktif"). Ini bukan insiden.

**Penanganan:** isi kedua variabel di `.env`, restart service notification.
**Verifikasi:** lakukan aksi yang memicu notifikasi (mis. checkout) dan cek chat
admin.

---

## 13. Insiden: disk penuh

**Gejala:** tulis gagal; Postgres/Kafka error; container restart.

**Diagnosis**
```bash
df -h
docker system df
du -sh data/uploads 2>/dev/null
```

**Penanganan**
```bash
docker system prune -f          # buang image/container tak terpakai
```
`backup.sh` otomatis membuang snapshot lokal lebih tua dari `BACKUP_KEEP_DAYS`
(default 7 hari). **Jangan** menghapus manual arsip yang masih dibutuhkan di
`BACKUP_REMOTE`. Untuk produksi, unggahan produk/bukti transfer berada di volume
`uploads`; besarnya dipantau bersama disk Postgres & Kafka.

---

## 14. Observability: Prometheus & Grafana

```bash
make observability        # Prometheus :9090, Grafana :3000
make observability-down
```

- Config scrape: `infra/observability/prometheus.yml` (via `host.docker.internal`).
- Dashboard: `infra/observability/grafana/dashboards/saybu.json` (8 panel).
- Metrik kunci: `http_requests_total`, `http_request_duration_seconds`,
  `grpc_requests_total`, `grpc_server_handling_seconds`, `kafka_consumer_lag`,
  `outbox_lag_seconds`, `orders_created_total`.
- Bila target `down`: pastikan service Go hidup dan port admin benar; cek
  `extra_hosts: host.docker.internal:host-gateway` ada di compose.

### Alerting

Aturan dan contact point di-provision otomatis dari
`infra/observability/grafana/provisioning/alerting/`：

| Alert | Kondisi | `for` | Severity |
|---|---|---|---|
| `saybu-service-down` | `up == 0` pada 7 job | 2m | critical |
| `saybu-http-5xx` | rasio 5xx gateway > 5% | 5m | warning |
| `saybu-kafka-lag` | lag consumer > 100 pesan | 10m | warning |
| `saybu-outbox-lag` | event outbox tertua > 300 detik | 5m | critical |

Cek aturan: `curl -u admin:admin http://localhost:3000/api/v1/provisioning/alert-rules`.
Variable `$TELEGRAM_BOT_TOKEN` dan `$TELEGRAM_ADMIN_CHAT_ID` diisi dari `.env` lewat compose.
Tanpa kredensial, provisioning memakai placeholder `disabled` supaya aturan tetap aktif;
pesan gagal dikirim dan tercatat di log Grafana. Setelah mengubah kredensial, restart Grafana
atau reload provisioning via API admin.

---

## 15. Load test

```bash
make load-test                                  # 100 VU katalog + 10 checkout/detik, 30s
DURATION=60s make load-test                     # durasi lain
API_BASE=http://host.docker.internal:8081 make load-test
```
Ambang: katalog p95 < 300 ms, checkout p95 < 500 ms, tingkat gagal katalog < 1%.
Script memakai `X-Forwarded-For` acak per iterasi agar batas baca per-IP tidak
menutupi kapasitas backend. Reset stok sebelum uji checkout:
`update products set stock = 100000;` pada `saybu_catalog`.

---

## 16. Rutin & eskalasi

- **Harian:** cek dashboard (error rate, lag, outbox), pastikan backup terbaru ada.
- **Mingguan:** uji restore ke snapshot terbaru (§9); tinjau DLQ; cek disk.
- **Eskalasi:** bila data hilang tanpa backup atau Kafka korup berkepanjangan,
  hentikan penulisan, dokumentasikan waktu kejadian, dan pulihkan dari snapshot
  terakhir sebelum menyelidiki akar masalah.

---

## 17. Deploy produksi (M7)

**Prasyarat di VPS:** Docker + Compose v2, firewall 22/80/443, repo di
`DEPLOY_PATH` (mis. `/opt/saybu`), `.env` produksi sudah diisi, dan cron backup
dipasang (`crontab infra/cron/saybu-backup.cron`). Evaluasi awal:
`bash infra/scripts/check-vps.sh`. Detail input yang masih dibutuhkan ada di
`docs/DEPLOY_CHECKLIST.md`.

**Secret GitHub Actions:** `SSH_HOST`, `SSH_USER`, `SSH_KEY`, `SSH_PORT` (opsional),
`DEPLOY_PATH`. Push ke `main` menjalankan `.github/workflows/cd.yml`.

**Deploy manual dari VPS (bila image sudah tersedia):**
```bash
cd /opt/saybu
export $(grep -v '^#' .env | xargs)   # atau: set -a; . ./.env; set +a
make prod-config                       # validasi compose
make prod-migrate                      # goose up, sekali jalan
make prod-up                           # pull sudah dilakukan CD; start/restart
make prod-logs
curl -sS https://<domain>/api/v1/version
curl -sS -o /dev/null -w '%{http_code}\n' https://<domain>/healthz
```

**Verifikasi setelah deploy**
```bash
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml --profile observability up -d prometheus grafana
# Grafana: folder "Saybu Alerts" → contact point telegram-admin
cat backups/$(ls -1 backups | tail -1)/manifest.txt
```

**Rollback cepat:** set `IMAGE_TAG=<sha-sebelumnya>` di `.env` VPS lalu ulangi
`make prod-migrate` (kecuali perubahan skema tidak kompatibel) dan `make prod-up`.
Bila skema berubah, restore dari snapshot terakhir terlebih dahulu (§9).

**Catatan:** domain/DNS, sertifikat, dan seed produk asli bukan bagian dari workflow;
lihat `docs/TASK.md` M7-01/03/09.

## 18. Uji produksi lokal (tanpa VPS/GHCR/domain)

Bisa dijalankan di mesin development yang sudah punya Docker + Compose v2. Memakai
artefak produksi yang sama, tetapi image di-build lokal, HTTP di `http://localhost`,
dan notifikasi Telegram dinonaktifkan. Berguna untuk memvalidasi perubahan sebelum VPS tersedia.

**Prasyarat:** `frontend/dist` sudah ter-build (`npm --prefix frontend run build`) dan
`.env.prod.local` berisi nilai dummy (ter-gitignore). Jangan pakai secret produksi di sini.

```bash
make local-prod-build     # build 7 service + image migrate dari Dockerfile
make local-prod-up        # start stack, container berprefix saybu-local-*
make local-prod-migrate   # goose up ke 6 DB (sekali setelah volume baru)
make local-prod-seed      # seed idempoten: 2 kategori, 12 produk, admin, 4 zona
make local-prod-logs      # opsional: ikuti log
make local-prod-down      # hentikan stack
```

**Alamat:** `http://localhost` (SPA + API), `/healthz`, `/readyz`, `/admin`.
Postgres host `localhost:5433`, Kafka `localhost:29092`. Akun admin seed:
`admin@saybu.test` / `admin12345`.

**Alur yang sudah diverifikasi (2026-11-26):**
- Checkout PICKUP COD → payment `COD_PENDING`; pelacakan via `?phone=`;
  status `PENDING_PAYMENT → PAID → PREPARING → READY_PICKUP → COMPLETED`
  membuat payment menjadi `COD_PAID` lewat event `order.status_changed`.
- Checkout PICKUP TRANSFER → upload bukti PNG → `WAITING_VERIFICATION` →
  admin approve → `PAID` dan order ikut `PAID`.
- Akses `/orders/{code}/payment` untuk guest **wajib** `?phone=` yang cocok;
  tanpa/phone salah sengaja 404 agar tidak membocorkan pembayaran orang lain.
- `/uploads/*` dilayani Caddy dari volume bersama gateway; `uri strip_prefix /uploads`
  wajib ada supaya path tidak dobel. `/healthz` dan `/readyz` juga harus punya
  `handle` sendiri sebelum SPA fallback.

**Batasan lokal:** tanpa TLS, tanpa GHCR, tanpa notifikasi Telegram (event masuk DLQ
dengan error `telegram: status 404` sampai token diisi), dan tanpa backup ke object
storage. Ini bukan pengganti verifikasi produksi di VPS.

---

## Referensi

- `docs/ARSITEKTUR.md` §12 (Observability), §13 (Deployment)
- `docs/SECURITY.md`
- `infra/scripts/{backup.sh,restore.sh,migrate.sh,dev.sh}`
- `infra/observability/`, `infra/load/k6-saybu.js`
- `docker-compose.prod.yml`, `infra/Caddyfile`, `backend/Dockerfile.migrate`
- `.github/workflows/cd.yml`, `infra/cron/saybu-backup.cron`
