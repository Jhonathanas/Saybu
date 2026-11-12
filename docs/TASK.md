# TASK — Saybu

| Field | Value |
|---|---|
| Versi | 1.0 |
| Terakhir diperbarui | 2026-11-11 |
| Cara pakai | Centang `[x]` saat selesai. Satu task = satu PR/commit yang bisa diuji. |

**Legenda status:** `[ ]` belum · `[~]` sedang dikerjakan · `[x]` selesai · `[!]` terblokir

---

## Ringkasan progres

| Milestone | Fokus | Task | Selesai | Status |
|---|---|---|---|---|
| M0 | Fondasi | 12 | 12 | ✅ |
| M1 | Auth & gateway | 13 | 0 | ⬜ |
| M2 | Katalog | 15 | 0 | ⬜ |
| M3 | Ongkir & order | 16 | 0 | ⬜ |
| M4 | Pembayaran & notifikasi | 14 | 0 | ⬜ |
| M5 | Kelengkapan | 12 | 0 | ⬜ |
| M6 | Observability & hardening | 8 | 0 | ⬜ |
| M7 | Deploy | 10 | 0 | ⬜ |

---

## M0 — Fondasi

| ID | Task | Acceptance | Est |
|---|---|---|---|
| M0-01 | `[x]` Inisialisasi repo: `git init`, `.gitignore`, `README.md`, `LICENSE` (opsional) | `git status` bersih, README berisi cara jalan | 0.5 j |
| M0-02 | `[x]` `docker-compose.yml`: Postgres 17, Kafka KRaft, Kafka UI, volume, healthcheck | `docker compose ps` semua `healthy` | 0.5 j |
| M0-03 | `[x]` Skrip init Postgres membuat 5 DB (`saybu_auth`, `catalog`, `shipping`, `orders`, `payments`) + user | `psql -l` menampilkan 5 DB | 0.5 j |
| M0-04 | `[x]` `backend/go.mod` (satu module `saybu`) + 7 binary service + `pkg` bersama | `go build ./...` dari `backend/` sukses | 0.5 j |
| M0-05 | `[x]` `pkg/config`, `pkg/logger`, `pkg/httpx`, `pkg/ids`, `pkg/money`, `pkg/telemetry` | Unit test dasar lulus | 1 j |
| M0-06 | `[x]` `pkg/grpcx`: interceptor request-id, logging, recovery, metrik + `pkg/grpcapp` | Test interceptor (bufconn) lulus | 1 j |
| M0-07 | `[x]` `pkg/kafkax` + `pkg/outbox`: producer/consumer, retry, DLQ, relay | `make test-integration` lulus dengan Kafka container | 1.5 j |
| M0-08 | `[x]` `backend/proto` + `buf.yaml`/`buf.gen.yaml`, generate ke `backend/gen` (di-commit) | `make proto` menghasilkan kode, `buf lint` bersih | 1 j |
| M0-09 | `[x]` Skeleton 7 service: `main.go`, config, graceful shutdown, `/healthz`, `/readyz`, `/metrics` | Semua service start & health check OK | 1.5 j |
| M0-10 | `[x]` `Makefile`: up, down, logs, ps, proto, migrate, seed, dev, test, test-integration, lint, fmt, build | Semua target berjalan | 0.5 j |
| M0-11 | `[x]` CI GitHub Actions: vet, test, build, buf lint/generate, compose config | Workflow tervalidasi lokal; hijau di PR pertama | 1 j |
| M0-12 | `[x]` `.env.example` + panduan dev di README (prasyarat, langkah, troubleshooting) | Orang baru bisa jalan dalam < 15 menit | 0.5 j |

**Gate M0:** `make up && make proto && make test` hijau, semua health check OK. ✅

**Catatan penyimpangan M0:**
- Satu Go module (`backend/go.mod`), bukan `go.work` multi-modul — build/CI/Docker lebih sederhana; service tetap binary & container terpisah.
- Go naik ke **1.26** karena dependency (franz-go/pgx terbaru) mensyaratkan toolchain tersebut.
- Postgres host **5433** (5432 dipakai container lain) dan Kafka UI **8090** (8080 bentrok di Windows).
- `make` dipasang lokal ke `~/.local/bin` karena sudo tidak tersedia.
- Timeout gRPC diatur lewat deadline dari pemanggil (tidak ada interceptor timeout terpisah).

---

## M1 — Auth & Gateway

| ID | Task | Acceptance | Est |
|---|---|---|---|
| M1-01 | `[ ]` Proto `auth.v1` + generate | `make proto` sukses, tidak ada breaking change | 0.5 j |
| M1-02 | `[ ]` Migrasi `saybu_auth`: `users`, `refresh_tokens` + index unik | Migrasi naik/turun bersih | 0.5 j |
| M1-03 | `[ ]` `sqlc` query auth + repo | Test repo dengan testcontainers | 1 j |
| M1-04 | `[ ]` Domain auth: validasi HP/email, aturan password, argon2id | Unit test ≥ 80% | 1 j |
| M1-05 | `[ ]` Usecase register/login/refresh rotasi/logout + deteksi reuse | Test reuse token mencabut semua sesi | 1.5 j |
| M1-06 | `[ ]` gRPC server auth + interceptor | Test `bufconn` | 1 j |
| M1-07 | `[ ]` Gateway: chi router, config, CORS, request-id, error mapping | Test mapping error | 1 j |
| M1-08 | `[ ]` JWT middleware + klien gRPC per service (connection reuse) | Token invalid → 401, role kurang → 403 | 1 j |
| M1-09 | `[ ]` Handler REST auth + `/me` + rate limit login | Sesuai `API_SPEC.md` §2 | 1 j |
| M1-10 | `[ ]` Scaffold frontend: Vite, TS, Tailwind, shadcn, router, QueryClient | `npm run dev` tampil | 1 j |
| M1-11 | `[ ]` API client + auth context + auto refresh 401 | Refresh hanya sekali, tidak loop | 1.5 j |
| M1-12 | `[ ]` Halaman login & register + validasi form | Bisa daftar & masuk | 1.5 j |
| M1-13 | `[ ]` Guard `/admin` + layout dasar admin | Customer ditolak, admin masuk | 0.5 j |

**Gate M1:** register → login → refresh → logout berjalan; `/admin` terlindungi; test & CI hijau.

---

## M2 — Katalog

| ID | Task | Acceptance | Est |
|---|---|---|---|
| M2-01 | `[ ]` Proto `catalog.v1` (termasuk `CheckStock`/`ReserveStock`/`ReleaseStock`) | `buf lint` bersih | 0.5 j |
| M2-02 | `[ ]` Migrasi `saybu_catalog`: `categories`, `products`, `product_images`, `stock_reservations` | Naik/turun bersih | 1 j |
| M2-03 | `[ ]` `sqlc` query katalog + repo | Test repo | 1 j |
| M2-04 | `[ ]` Domain katalog: stok, reservasi, aturan harga, slug unik | Unit test | 1 j |
| M2-05 | `[ ]` Usecase produk (CRUD, set/adjust stok) + validasi | Test usecase | 1.5 j |
| M2-06 | `[ ]` gRPC server catalog | Test `bufconn` | 1 j |
| M2-07 | `[ ]` Gateway: endpoint publik kategori & produk + filter/pagination | Sesuai `API_SPEC.md` §3 | 1 j |
| M2-08 | `[ ]` Gateway: endpoint admin produk + upload gambar (validasi tipe/ukuran) | Upload > 5 MB ditolak | 1.5 j |
| M2-09 | `[ ]` Seed: 2 kategori + 12 produk + zona + 1 admin | `make seed` idempoten | 1 j |
| M2-10 | `[ ]` Storefront: home (kategori + produk unggulan) | Mobile rapi 360 px | 1 j |
| M2-11 | `[ ]` Storefront: daftar produk + filter kategori + pencarian | Filter & cari bekerja | 1.5 j |
| M2-12 | `[ ]` Storefront: detail produk (galeri, harga, stok, pilih jumlah) | Stok 0 tidak bisa ditambah | 1.5 j |
| M2-13 | `[ ]` Cart store (Zustand + persist) + badge & halaman keranjang | Bertahan setelah refresh | 1 j |
| M2-14 | `[ ]` Admin: tabel produk + form create/edit + upload foto + ubah stok | CRUD penuh dari UI | 2 j |
| M2-15 | `[ ]` Test frontend (Vitest) untuk cart & format uang | `npm test` hijau | 1 j |

**Gate M2:** produk tampil & bisa dicari, admin CRUD + upload jalan, stok tercermin di storefront.

---

## M3 — Ongkir, Checkout, Order, Kafka

| ID | Task | Acceptance | Est |
|---|---|---|---|
| M3-01 | `[ ]` Service `shipping`: proto, migrasi `zones`, repo, domain kuotasi | Unit + repo test | 1.5 j |
| M3-02 | `[ ]` Gateway: zona publik + quote + CRUD zona admin | Sesuai `API_SPEC.md` §4 | 1 j |
| M3-03 | `[ ]` Proto `order.v1` + migrasi `orders`, `order_items`, `order_status_history`, `idempotency_keys`, `outbox` | Naik/turun bersih | 1.5 j |
| M3-04 | `[ ]` Domain order: state machine + validasi transisi | Test semua transisi legal/ilegal | 1.5 j |
| M3-05 | `[ ]` Usecase checkout: quote ongkir → cek stok → reserve → simpan order → outbox | Test kompensasi saat gagal simpan | 2 j |
| M3-06 | `[ ]` Idempotensi checkout (`Idempotency-Key`, hash body) | Double submit → satu order | 1 j |
| M3-07 | `[ ]` `pkg/outbox` relay di order + producer Kafka | Event muncul di Kafka UI | 1.5 j |
| M3-08 | `[ ]` Gateway: `POST /orders`, `GET /orders/{code}`, cancel | Sesuai `API_SPEC.md` §5 | 1.5 j |
| M3-09 | `[ ]` UI checkout: data pembeli, pilih antar/ambil, zona, metode bayar, ringkasan | Total = subtotal + ongkir | 2 j |
| M3-10 | `[ ]` UI halaman sukses + instruksi bayar + kode pesanan | Kode tampil & bisa disalin | 1 j |
| M3-11 | `[ ]` UI lacak pesanan (kode + no. HP) | Status & riwayat tampil | 1 j |
| M3-12 | `[ ]` Snapshot item & alamat di order | Ubah harga produk tidak mengubah order lama | 0.5 j |
| M3-13 | `[ ]` Validasi server: harga, stok, step qty, min qty, zona | Test kasus negatif | 1.5 j |
| M3-14 | `[ ]` Integration test alur checkout dengan Kafka container | Lulus di CI | 2 j |
| M3-15 | `[ ]` Consumer `order.events.v1` di catalog (release stok) | Batal → stok kembali | 1 j |
| M3-16 | `[ ]` Job expire order 24 jam → `order.expired` | Order lama otomatis expired | 1 j |

**Gate M3:** checkout end-to-end membuat order + event Kafka; stok ter-reserve; batal melepas stok; tidak ada oversell.

---

## M4 — Pembayaran & Notifikasi

| ID | Task | Acceptance | Est |
|---|---|---|---|
| M4-01 | `[ ]` Proto `payment.v1` + migrasi `saybu_payments` (`payments`, `payment_proofs`, `processed_events`) | Naik/turun bersih | 1 j |
| M4-02 | `[ ]` Usecase buat payment saat `order.created` + status awal | Consumer idempoten | 1.5 j |
| M4-03 | `[ ]` Upload bukti (validasi tipe/ukuran, simpan ke volume, URL) | File tersimpan & bisa diakses | 1.5 j |
| M4-04 | `[ ]` Verifikasi admin (approve/reject) + event | Event `payment.confirmed` terbit | 1 j |
| M4-05 | `[ ]` Consumer `payment.events.v1` di order (ubah status) | Status order ikut berubah | 1 j |
| M4-06 | `[ ]` Consumer `order.events.v1` di payment (batal/expired) | Payment ditandai batal | 0.5 j |
| M4-07 | `[ ]` Service notification: consumer + klien Telegram + retry + DLQ | Pesan masuk Telegram | 1.5 j |
| M4-08 | `[ ]` Template pesan: order baru, bukti masuk, terverifikasi, ditolak, batal | Format rapi & informatif | 1 j |
| M4-09 | `[ ]` UI instruksi transfer + form upload bukti | Guest bisa upload dengan kode+HP | 1.5 j |
| M4-10 | `[ ]` UI admin: daftar pembayaran menunggu + preview bukti + approve/reject | Verifikasi dari HP | 1.5 j |
| M4-11 | `[ ]` UI status pembayaran di halaman lacak | Status jelas & real-time-ish | 0.5 j |
| M4-12 | `[ ]` Idempotensi consumer (`processed_events`) di semua consumer | Kirim event duplikat → efek sekali | 1 j |
| M4-13 | `[ ]` DLQ + perintah replay manual | Pesan gagal bisa diproses ulang | 1 j |
| M4-14 | `[ ]` Test integrasi pembayaran penuh | Lulus di CI | 2 j |

**Gate M4:** transfer manual end-to-end (upload → verifikasi → order PAID) + Telegram berfungsi, duplikasi aman.

---

## M5 — Kelengkapan

| ID | Task | Acceptance | Est |
|---|---|---|---|
| M5-01 | `[ ]` Riwayat pesanan untuk akun (`/me/orders`) | Hanya pesanan sendiri | 1 j |
| M5-02 | `[ ]` Admin: daftar & detail pesanan + filter status/tanggal/cari | Filter akurat | 1.5 j |
| M5-03 | `[ ]` Admin: ubah status dari UI dengan validasi | Transisi ilegal ditolak | 1 j |
| M5-04 | `[ ]` Admin: dashboard ringkasan hari ini | Angka sesuai DB | 1 j |
| M5-05 | `[ ]` Pembatalan oleh pembeli + konfirmasi | Stok dilepas | 1 j |
| M5-06 | `[ ]` COD: tandai lunas saat selesai | Status pembayaran COD benar | 0.5 j |
| M5-07 | `[ ]` Notifikasi stok menipis ke admin | Event `stock.low` terkirim sekali per ambang | 1 j |
| M5-08 | `[ ]` Halaman 404/500 + error boundary frontend | Tidak blank screen | 0.5 j |
| M5-09 | `[ ]` Empty states & loading skeleton | UX rapi | 0.5 j |
| M5-10 | `[ ]` SEO dasar: title, meta, OG image, sitemap | Terbaca crawler | 0.5 j |
| M5-11 | `[ ]` Audit log admin sederhana | Perubahan tercatat | 1 j |
| M5-12 | `[ ]` Uji usability panel dengan orangtua | Ada catatan perbaikan | 1 j |

**Gate M5:** seluruh alur bisnis v1 lengkap dan bisa dipakai sehari-hari.

---

## M6 — Observability & hardening

| ID | Task | Acceptance | Est |
|---|---|---|---|
| M6-01 | `[ ]` Metrik Prometheus semua service | `/metrics` berisi metrik kunci | 1 j |
| M6-02 | `[ ]` Dashboard Grafana (request rate, latency, error, consumer lag, outbox lag) | Dashboard bisa dipakai | 1.5 j |
| M6-03 | `[ ]` Propagasi `request_id` HTTP → gRPC → event → log | Satu request bisa dilacak | 1 j |
| M6-04 | `[ ]` Rate limit seluruh endpoint sesuai spec | 429 muncul saat melebihi | 0.5 j |
| M6-05 | `[ ]` Security review: CORS, header, secret, upload, SQL injection | Checklist aman | 1 j |
| M6-06 | `[ ]` Load test k6 (100 VU katalog + 10 checkout/detik) | p95 < 300 ms lokal | 1 j |
| M6-07 | `[ ]` Uji restore backup | Data kembali utuh | 1 j |
| M6-08 | `[ ]` Dokumentasi operasional (runbook) | Ada panduan insiden | 1 j |

---

## M7 — Deploy

| ID | Task | Acceptance | Est |
|---|---|---|---|
| M7-01 | `[ ]` Provision VPS ≥ 4 GB + Docker + firewall (22/80/443) | `docker run hello-world` | 1 j |
| M7-02 | `[ ]` `docker-compose.prod.yml` + Caddy (TLS, SPA fallback, uploads) | HTTPS aktif | 1.5 j |
| M7-03 | `[ ]` Domain + DNS + email admin | Domain resolve | 0.5 j |
| M7-04 | `[ ]` Dockerfile semua service (multi-stage, non-root, healthcheck) | Image < 50 MB per service | 1.5 j |
| M7-05 | `[ ]` GitHub Actions CD ke GHCR + deploy SSH | Push ke main → live | 1 j |
| M7-06 | `[ ]` Job migrasi sebelum deploy service | Migrasi otomatis & aman | 0.5 j |
| M7-07 | `[ ]` Backup harian Postgres + upload object storage + retensi 7 hari | Backup berjalan otomatis | 1 j |
| M7-08 | `[ ]` Monitoring uptime + alert Telegram bila service down | Alert diterima | 0.5 j |
| M7-09 | `[ ]` Seed data produk asli + foto + zona ongkir | Katalog siap jual | 1 j |
| M7-10 | `[ ]` Go-live checklist: uji pesanan nyata, verifikasi transfer, notifikasi | Semua lulus | 1 j |

---

## Definition of Done (setiap task)

- [ ] Kode + test (unit/integrasi sesuai kebutuhan) lulus
- [ ] `make lint` dan `make test` hijau
- [ ] Kontrak diperbarui di `API_SPEC.md` / `ARSITEKTUR.md` bila berubah
- [ ] Tidak ada secret/credential di commit
- [ ] Error & log tidak membocorkan data sensitif
- [ ] Perubahan bisa diuji manual dengan langkah yang jelas
- [ ] `TASK.md` dicentang + catatan bila ada penyimpangan

---

## Backlog ide (belum dijadwalkan)

- Kupon diskon & minimum belanja
- Ongkir berbasis radius (Haversine) + pin lokasi
- Notifikasi WhatsApp (Fonnte/Wablas)
- Laporan penjualan mingguan/bulanan + ekspor CSV
- Program langganan sayur mingguan
- Multi-bahasa (ID/EN)
- PWA + mode offline katalog
- Rekomendasi produk & "sering dibeli bersama"
- Integrasi payment gateway (QRIS)
- Read model laporan lewat consumer Kafka

---

## Log keputusan & catatan

| Tanggal | Catatan |
|---|---|
| 2026-11-11 | Dokumen awal dibuat: PRD, arsitektur, tech stack, API spec, task, AGENTS.md |
| 2026-11-11 | Keputusan: Kafka, gRPC internal, React SPA, admin satu app, deploy lokal dulu |
| 2026-11-11 | **M0 selesai.** Toolchain Linux Go 1.26 + buf 1.75 dipasang; infra sehat (5 DB, 8 topic); 7 service lulus health/ready; unit + integrasi Kafka hijau |
| 2026-11-11 | Penyimpangan: satu Go module (bukan go.work), Postgres host 5433, Kafka UI 8090, Go 1.26 |
