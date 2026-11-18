# ARSITEKTUR — Saybu

| Field | Value |
|---|---|
| Versi | 1.0 |
| Status | Baseline untuk M0–M2, ditinjau ulang di M3 |
| Terakhir diperbarui | 2026-11-11 |

---

## 1. Prinsip arsitektur

1. **Gateway tunggal** — hanya `gateway` yang terekspos ke browser. Service lain hanya di jaringan internal.
2. **Database per service** — tidak ada service membaca DB service lain. Integrasi lewat gRPC atau event.
3. **Event-driven untuk efek samping** — notifikasi, pelepasan stok, dan perubahan status lintas service memakai Kafka.
4. **Transactional outbox** — event ditulis di transaksi DB yang sama dengan perubahan data, lalu direlay. Tidak ada event hilang / hantu.
5. **Idempotent by default** — consumer memakai `event_id`, endpoint tulis memakai `Idempotency-Key`.
6. **Kontrak dulu** — protobuf untuk internal, OpenAPI untuk publik.
7. **Stateless service** — state hanya di Postgres/Kafka/volume upload.
8. **Observable** — setiap request punya `request_id`, setiap service punya `/healthz`, `/readyz`, `/metrics`.
9. **Uang sebagai `int64` rupiah**, stok sebagai bilangan bulat satuan terkecil (gram/pcs).

---

## 2. Diagram container

```
                          ┌──────────────────────────────┐
                          │  Browser (HP / desktop)      │
                          │  React SPA + /admin          │
                          └──────────────┬───────────────┘
                                         │ HTTPS /api/v1/*
                          ┌──────────────▼───────────────┐
                          │  Caddy (TLS, static, upload) │
                          └───────┬──────────────┬───────┘
                                  │ /            │ /api
                       ┌──────────▼───┐   ┌──────▼──────────────┐
                       │ static files │   │  API Gateway (Go)   │
                       │ (React build)│   │  JWT, CORS, rate    │
                       └──────────────┘   └──┬───┬───┬───┬───┬──┘
                                             │gRPC (internal) │
              ┌──────────────────────────────┼───┼───┼───┼───┼───────────────┐
              │                              │   │   │   │   │               │
        ┌─────▼─────┐  ┌──────────────┐  ┌───▼───▼┐  │  ┌▼──────────┐  ┌─────▼─────────┐
        │   auth    │  │   catalog    │  │ order  │  │  │  payment  │  │   shipping    │
        │  :50051   │  │   :50052     │  │ :50054 │  │  │  :50055   │  │   :50053      │
        └─────┬─────┘  └──────┬───────┘  └───┬────┘  │  └────┬──────┘  └──────┬────────┘
              │               │              │       │       │                │
        ┌─────▼─────┐  ┌──────▼───────┐  ┌───▼────┐  │  ┌────▼──────┐  ┌──────▼────────┐
        │ saybu_auth│  │saybu_catalog │  │saybu_  │  │  │saybu_     │  │saybu_shipping │
        │ (Postgres)│  │  (Postgres)  │  │orders  │  │  │payments   │  │  (Postgres)   │
        └───────────┘  └──────────────┘  └───┬────┘  │  └────┬──────┘  └───────────────┘
                                              │       │       │
                                              └───────┴───────┴──────┐
                                                                     │ publish/consume
                                              ┌──────────────────────▼─────────────────┐
                                              │            Kafka (KRaft)               │
                                              │  order.events.v1                      │
                                              │  payment.events.v1                    │
                                              │  catalog.events.v1                    │
                                              │  notification.commands.v1             │
                                              │  *.dlq                                │
                                              └──────────────────────┬─────────────────┘
                                                                     │ consume
                                                          ┌──────────▼──────────┐
                                                          │ notification        │
                                                          │ :8086 (Telegram)    │
                                                          └─────────────────────┘
```

---

## 3. Katalog service

| Service | Port gRPC | Port admin (health/metrics) | Database | Tanggung jawab | Panggilan sinkron keluar | Event diproduksi | Event dikonsumsi |
|---|---|---|---|---|---|---|---|
| `gateway` | — (REST :8081) | :8081 | — | Routing, validasi JWT, CORS, rate limit, reverse ke gRPC | semua service | — | — |
| `auth` | 50051 | 9101 | `saybu_auth` | Registrasi, login, refresh, role, profil | — | — | — |
| `catalog` | 50052 | 9102 | `saybu_catalog` | Produk, kategori, stok, reservasi stok | — | `catalog.events.v1` | `order.events.v1` (release stok) |
| `shipping` | 50053 | 9103 | `saybu_shipping` | Zona & tarif ongkir, kuotasi | — | — | — |
| `order` | 50054 | 9104 | `saybu_orders` | Keranjang server, checkout, state machine pesanan, outbox | catalog (reserve), shipping (quote) | `order.events.v1` | `payment.events.v1` |
| `payment` | 50055 | 9105 | `saybu_payments` | Bukti transfer, verifikasi, status bayar | order (status) | `payment.events.v1` | `order.events.v1` |
| `notification` | — | :8086 | — | Konsumen event → Telegram | — | — | semua event & `notification.commands.v1` |

**Aturan kepemilikan data**

| Data | Pemilik | Disalin ke |
|---|---|---|
| User & kredensial | `auth` | — |
| Produk, harga, stok | `catalog` | snapshot di `orders.order_items` |
| Zona & tarif | `shipping` | snapshot di `orders.shipping_fee`, `orders.zone_name` |
| Pesanan & status | `order` | — |
| Pembayaran | `payment` | — |

> Snapshot wajib: perubahan harga produk besok tidak boleh mengubah total pesanan kemarin.

---

## 4. Komunikasi

### 4.1 Sinkron — gRPC

- Semua service internal mengekspos gRPC (HTTP/2, tanpa TLS di jaringan internal; mTLS fase M6+).
- Gateway memegang **satu** koneksi per service (connection reuse), bukan per request.
- Deadline default **2 detik** untuk read, **5 detik** untuk checkout.
- Retry hanya untuk panggilan **idempoten** (`Get*`, `List*`, `CheckStock`) dengan backoff 50–200 ms, maks 2x.
- Interceptor wajib: `request_id`, logging, recovery (ubah panic → `codes.Internal`), timeout, auth context.
- Metadata diteruskan gateway: `x-user-id`, `x-user-role`, `x-request-id`.
- Error gRPC dipetakan ke HTTP di gateway (lihat `API_SPEC.md` §2).

### 4.2 Asinkron — Kafka

- Semua efek samping lintas service lewat Kafka, **bukan** panggilan sinkron berantai.
- Producer memakai idempotent producer (`enable.idempotence=true`, `acks=all`).
- Consumer commit setelah sukses; kegagalan → retry in-process (3x, backoff eksponensial 1s/2s/4s) → topik `.dlq`.
- Consumer wajib idempotent: tabel `processed_events(event_id PRIMARY KEY)` di DB service masing-masing.
- Consumer group: satu per service (`catalog-svc`, `order-svc`, `payment-svc`, `notification-svc`).

---

## 5. Desain Kafka

### 5.1 Cluster

| Item | Nilai lokal | Catatan produksi |
|---|---|---|
| Mode | KRaft single-node | KRaft, 1 broker cukup |
| Replication factor | 1 | 1 (single VPS) |
| `min.insync.replicas` | 1 | 1 |
| Partitions | 3 per topic | 3 |
| Retention | 7 hari | 7 hari |
| Heap | 512 MB–1 GB | sesuaikan RAM VPS |

### 5.2 Topik

| Topic | Key | Producer | Consumer group | Isi |
|---|---|---|---|---|
| `order.events.v1` | `order_id` | order | catalog, payment, notification | `order.created`, `order.cancelled`, `order.status_changed`, `order.expired` |
| `payment.events.v1` | `order_id` | payment | order, notification | `payment.proof_uploaded`, `payment.confirmed`, `payment.rejected` |
| `catalog.events.v1` | `product_id` | catalog | notification | `product.updated`, `stock.low` (saat checkout menembus ambang), `stock.released` |
| `notification.commands.v1` | `recipient_key` | service mana pun | notification | perintah kirim pesan (Telegram) |
| `<topic>.dlq` | sama | consumer | — | pesan gagal permanen |

### 5.3 Envelope event

```json
{
  "event_id": "0192f3c4-...",
  "type": "order.created",
  "version": 1,
  "occurred_at": "2026-11-11T10:15:30.123Z",
  "aggregate_id": "0192f3c4-order-uuid",
  "correlation_id": "req-uuid",
  "producer": "order-svc",
  "payload": { }
}
```

Aturan evolusi: **hanya aditif**. Field tidak boleh dihapus/diubah tipe. Perubahan besar → `v2`.

### 5.4 Payload penting

```jsonc
// order.created
{
  "order_id": "...", "code": "SB-20261111-0007",
  "customer_name": "Siti", "customer_phone": "0812...",
  "fulfillment": "DELIVERY",            // DELIVERY | PICKUP
  "payment_method": "TRANSFER",         // TRANSFER | COD
  "subtotal": 75000, "shipping_fee": 10000, "total": 85000,
  "items": [{ "product_id": "...", "name": "Bayam", "unit": "ikat", "qty": 2, "unit_price": 5000 }]
}

// payment.confirmed
{ "payment_id": "...", "order_id": "...", "amount": 85000, "verified_by": "admin-uuid" }

// stock.low — dipancarkan saat stok melewati ambang (sekali per pelanggaran)
{ "product_id": "...", "name": "Tomat", "remaining": 3, "unit": "kg", "threshold": 5 }
```

---

## 6. Transactional outbox

```
┌─────────────────────────── transaksi DB ───────────────────────────┐
│ 1. UPDATE/INSERT data bisnis (mis. orders, order_items)            │
│ 2. INSERT INTO outbox (event_id, topic, key, payload, headers)     │
│ 3. COMMIT                                                          │
└────────────────────────────────────────────────────────────────────┘
                 │
                 ▼
   relay goroutine (tiap 200 ms, batch 100)
   SELECT ... FROM outbox WHERE published_at IS NULL
   ORDER BY id FOR UPDATE SKIP LOCKED
                 │
                 ▼
   Kafka produce (acks=all, idempotent)
                 │
                 ▼
   UPDATE outbox SET published_at = now()
```

- Pengiriman **at-least-once**; duplikasi ditangani consumer lewat `event_id`.
- Gagal kirim → baris tetap `published_at IS NULL`, dicoba lagi; metrik `outbox_lag_seconds`.
- Tabel: `outbox(id bigserial, event_id uuid unique, topic text, key text, payload jsonb, headers jsonb, created_at, published_at, attempts int, last_error text)`.

---

## 7. Saga & state machine pesanan

### 7.1 Alur checkout

```
Client ──POST /orders──► gateway ──gRPC──► order
                                            │ 1. shipping.Quote(zone, subtotal)
                                            │ 2. catalog.CheckStock(items)
                                            │ 3. catalog.ReserveStock(items)  ← sinkron, gagal → tolak
                                            │ 4. INSERT order + items (PENDING_PAYMENT)
                                            │ 5. INSERT outbox order.created
                                            └──► response { code, total, ... }
```

- Reservasi stok dilakukan **sinkron** agar oversell dicegah di depan.
- Jika langkah 4 gagal setelah reservasi → `catalog.ReleaseStock` (kompensasi).
- Jika pembayaran tidak selesai dalam 24 jam → job di order-service → `order.expired` → catalog melepas stok.

### 7.2 Status

| Status | Arti | Transisi berikutnya |
|---|---|---|
| `PENDING_PAYMENT` | dibuat, belum dibayar | `WAITING_VERIFICATION`, `PAID` (COD), `CANCELLED`, `EXPIRED` |
| `WAITING_VERIFICATION` | bukti transfer diunggah | `PAID`, `PENDING_PAYMENT` (ditolak), `CANCELLED` |
| `PAID` | pembayaran sah | `PREPARING`, `CANCELLED` |
| `PREPARING` | sedang disiapkan | `SHIPPED`, `READY_PICKUP`, `CANCELLED` |
| `SHIPPED` | sedang diantar | `COMPLETED` |
| `READY_PICKUP` | siap diambil | `COMPLETED` |
| `COMPLETED` | selesai | — |
| `CANCELLED` | dibatalkan | — |
| `EXPIRED` | kedaluwarsa 24 jam | — |

Transisi divalidasi di domain `order`; yang ilegal → `FAILED_PRECONDITION` → HTTP 409.

- Pembeli boleh membatalkan selama `PENDING_PAYMENT` / `WAITING_VERIFICATION` / `PAID` (stok dilepas).
- Admin boleh mengubah status lewat `PATCH /admin/orders/{id}/status` (transisi tervalidasi, tercatat di
  `order_status_history` dan memancarkan `order.status_changed`).
- COD: pembayaran dimulai `COD_PENDING`; saat pesanan `COMPLETED`, payment menandai `COD_PAID` dari
  event `order.status_changed` (M5-06).

### 7.3 Efek event

| Event | order | catalog | payment | notification |
|---|---|---|---|---|
| `order.created` | — | — | siapkan record | Telegram ke admin |
| `order.cancelled` | — | release stok | tandai batal | Telegram |
| `order.expired` | set status | release stok | tandai batal | Telegram |
| `payment.proof_uploaded` | `WAITING_VERIFICATION` | — | — | Telegram |
| `payment.confirmed` | `PAID` | kurangi stok permanen* | — | Telegram ke pembeli & admin |
| `payment.rejected` | `PENDING_PAYMENT` | — | — | Telegram |
| `order.status_changed` | — | — | COD → `COD_PAID` saat `COMPLETED` | Telegram |
| `stock.low` | — | — | — | Telegram ke admin |

\* Reservasi sudah mengurangi stok tersedia saat checkout; `payment.confirmed` mengonversi reservasi menjadi pemakaian final. Bila pesanan batal/expired, reservasi dilepas.

---

## 8. Konsistensi & batasan

- Konsistensi lintas service bersifat **eventual**; UI menampilkan status "memproses" bila perlu.
- Tidak ada distributed transaction (2PC). Kompensasi dilakukan lewat event/saga.
- Query lintas domain (mis. laporan) dilakukan di service pemilik atau lewat read model di fase lanjut.
- Nomor urut pesanan (`SB-YYYYMMDD-NNNN`) dibuat di order-service dengan sequence harian di DB (bukan Snowflake).

---

## 9. Idempotensi

| Lapisan | Mekanisme |
|---|---|
| Checkout | Header `Idempotency-Key` (UUID dari client) disimpan di `orders.idempotency_keys(key, order_id, request_hash, response_json, created_at)`; key sama → kembalikan respons lama |
| Consumer | `processed_events(event_id)` dicek & di-insert dalam transaksi yang sama dengan efeknya |
| Producer | Kafka idempotent producer + outbox `event_id` unik |
| Upload bukti | `payment_id` unik per order; upload ulang mengganti bukti lama |

---

## 10. Keamanan

- Password: **argon2id** (memory 64 MB, iterations 3, parallelism 2).
- JWT: akses 15 menit (HS256, secret dari env), refresh 30 hari disimpan **hash**-nya, rotasi setiap refresh, deteksi reuse → cabut semua sesi user.
- Gateway memvalidasi JWT & role; service internal percaya metadata dari gateway (jaringan privat).
- Rate limit (detail `API_SPEC.md` §10): login 5/menit per IP+identifier, register 3/menit per IP,
  checkout 10/menit per IP, unggah bukti 5/menit per IP, tulis admin 60/menit per user, baca 120/menit
  per IP. Respons 429 menyertakan `Retry-After`; bucket kedaluwarsa dibersihkan berkala.
- Header keamanan di gateway (`httpx.SecurityHeaders`): `X-Content-Type-Options: nosniff`,
  `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`,
  `Permissions-Policy`, dan `Strict-Transport-Security` (hanya di produksi).
- Validasi input di gateway (bentuk) dan service (bisnis).
- Upload: whitelist `image/jpeg`, `image/png`, `application/pdf`; maks 5 MB; nama file di-generate; tidak dieksekusi.
- CORS hanya origin frontend.
- Secret tidak pernah masuk repo; `.env` di-gitignore.
- Log tidak boleh memuat password, token, atau isi bukti transfer.

Checklist lengkap & tindak lanjut: `docs/SECURITY.md`.

---

## 11. Penyimpanan file

- Interface `storage.Storage` (`Save`, `Delete`, `URL`).
- Implementasi awal: disk lokal `./data/uploads/{yyyy}/{mm}/{uuid}.{ext}` pada volume Docker.
- Disajikan Caddy pada `/uploads/*` (read-only, `Cache-Control: public, max-age=31536000, immutable`).
- Abstraksi memungkinkan pindah ke S3/MinIO tanpa mengubah domain.

---

## 12. Observability

- Log JSON (`slog`) dengan field: `time`, `level`, `service`, `request_id`, `user_id`, `method`, `path`, `status`, `duration_ms`, `error`.
- `/healthz` (liveness, tanpa dependensi) dan `/readyz` (cek DB/Kafka).
- **Korelasi request (M6-03):** `request_id` dibuat/diambil di gateway (`X-Request-Id`), diteruskan ke
  gRPC lewat metadata, dan disimpan sebagai `correlation_id` pada setiap event Kafka
  (`kafkax.NewEventCtx`). Consumer menyemai `request_id` handler dari `correlation_id`
  (cadangan: `event_id`), sehingga satu request dapat dilacak HTTP → gRPC → event → log.
- `/metrics` Prometheus per service. Metrik kunci:
  | Metrik | Tipe | Arti |
  |---|---|---|
  | `saybu_<svc>_http_requests_total` | counter | Jumlah request HTTP per method/path/status |
  | `saybu_<svc>_http_request_duration_seconds` | histogram | Latensi HTTP |
  | `saybu_<svc>_grpc_requests_total` | counter | Jumlah request gRPC per method/code |
  | `saybu_<svc>_grpc_server_handling_seconds` | histogram | Latensi penanganan gRPC |
  | `saybu_<svc>_kafka_consumer_lag` | gauge | Lag consumer group per topic (via `kadm`) |
  | `saybu_<svc>_outbox_lag_seconds` | gauge | Usia event outbox tertua yang belum terkirim |
  | `saybu_<svc>_orders_created_total` | counter | Pesanan berhasil dibuat |
- Grafana + Prometheus dijalankan opsional di compose (profil `observability`, `make observability`).
  Prometheus men-scrape gateway `:8081` dan admin `:9101`–`:9105` serta notification `:8086` lewat
  `host.docker.internal`. Dashboard ter-provision: `infra/observability/grafana/dashboards/saybu.json`
  (8 panel: HTTP rate/p95/5xx, gRPC rate/p95, consumer lag, outbox lag, pesanan/menit).
- **Audit admin (M5):** middleware di gateway mencatat setiap mutasi admin yang berhasil
  (POST/PATCH/DELETE di bawah `/admin`) sebagai log terstruktur: `actor`, `action`, `path`, `status`,
  `request_id`. Tabel `audit_logs` persisten ditunda (lihat §15). Perubahan status pesanan tetap
  tersimpan di `order_status_history`.

Panduan penanganan insiden: `docs/RUNBOOK.md`.

---

## 13. Deployment

### Lokal (sekarang)

- `docker compose up -d` untuk Postgres, Kafka, Kafka UI.
- Service Go dijalankan langsung (`go run`) atau via `air` untuk hot reload.
- Frontend `npm run dev` (Vite), proxy `/api` → `:8081`.

### Produksi (M7)

- Satu VPS ≥ 4 GB RAM, 2 vCPU, Ubuntu LTS.
- Docker Compose: Caddy, frontend static, 7 service, Postgres, Kafka, Prometheus (opsional).
- Caddy: TLS otomatis (Let's Encrypt), reverse proxy `/api/*` → gateway, sisanya SPA fallback.
- GitHub Actions: build & push image ke GHCR → SSH ke VPS → `docker compose pull && up -d`.
- Backup: `pg_dump` harian per DB → kompres → unggah ke object storage; retensi 7 hari.
- Migrasi dijalankan sebagai job sekali jalan sebelum service baru aktif.

---

## 14. Architecture Decision Records (ringkas)

| ADR | Keputusan | Alasan | Konsekuensi |
|---|---|---|---|
| ADR-001 | Microservices (7 service) | Latihan & pemisahan domain jelas | Kompleksitas ops naik, butuh disiplin kontrak |
| ADR-002 | Kafka sebagai message broker | Target belajar, durable log, replay, ekosistem | Berat di RAM; butuh tuning |
| ADR-003 | gRPC untuk panggilan internal | Kontrak kuat, performa, codegen | Tooling protobuf, debugging lebih repot |
| ADR-004 | Postgres + sqlc | SQL eksplisit, tipe aman, ringan | Perlu generate ulang saat query berubah |
| ADR-005 | Transactional outbox | Menghindari event hilang/hantu | Tambah tabel & relay, latensi kecil |
| ADR-006 | Satu VPS + Compose | Murah, cukup untuk skala toko | Titik tunggal kegagalan; backup wajib |
| ADR-007 | React SPA (Vite) + REST di gateway | Sederhana, cepat, tidak perlu SSR | SEO terbatas (dapat ditingkatkan nanti) |
| ADR-008 | Snapshot data di pesanan | Riwayat pesanan stabil | Duplikasi data terkendali |

---

## 15. Jalur evolusi

- **Radius ongkir**: simpan lat/lng toko & alamat, hitung Haversine; butuh geocoding (opsional manual pin).
- **Payment gateway**: payment-service menambah provider; alur manual tetap ada.
- **Read model laporan**: consumer membangun tabel agregat harian.
- **gRPC-Web / Connect**: bila ingin menghapus gateway REST.
- **Redpanda**: pengganti Kafka hemat RAM, API kompatibel.
- **Multi-admin & audit log persisten**: tabel `audit_logs` + role granular (M5 baru mencatat audit mutasi admin sebagai log terstruktur di gateway).
