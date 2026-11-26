# TASK — Saybu

| Field | Value |
|---|---|
| Versi | 1.0 |
| Terakhir diperbarui | 2026-11-22 |
| Cara pakai | Centang `[x]` saat selesai. Satu task = satu PR/commit yang bisa diuji. |

**Legenda status:** `[ ]` belum · `[~]` sedang dikerjakan · `[x]` selesai · `[!]` terblokir

---

## Ringkasan progres

| Milestone | Fokus | Task | Selesai | Status |
|---|---|---|---|---|
| M0 | Fondasi | 12 | 12 | ✅ |
| M1 | Auth & gateway | 13 | 13 | ✅ |
| M2 | Katalog | 15 | 15 | ✅ |
| M3 | Ongkir & order | 16 | 16 | ✅ |
| M4 | Pembayaran & notifikasi | 14 | 14 | ✅ |
| M5 | Kelengkapan | 12 | 11 | 🟡 |
| M6 | Observability & hardening | 9 | 9 | ✅ |
| M7 | Deploy | 10 | 0 | ⬜ |

> **M5 = 11/12:** seluruh fitur selesai, tetapi `M5-12` (uji usability panel dengan orangtua)
> belum dijalankan karena butuh partisipan nyata. Gate M5 baru dianggap penuh setelah ada
> catatan hasil uji dan perbaikan kecilnya.

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
| M1-01 | `[x]` Proto `auth.v1` + generate | `make proto` sukses, tidak ada breaking change | 0.5 j |
| M1-02 | `[x]` Migrasi `saybu_auth`: `users`, `refresh_tokens` + index unik | Migrasi naik/turun bersih | 0.5 j |
| M1-03 | `[x]` Repo auth (pgx) — `sqlc` ditunda | Test repo integrasi lulus | 1 j |
| M1-04 | `[x]` Domain auth: validasi HP/email, aturan password, argon2id | Unit test ≥ 80% | 1 j |
| M1-05 | `[x]` Usecase register/login/refresh rotasi/logout + deteksi reuse | Test reuse token mencabut semua sesi | 1.5 j |
| M1-06 | `[x]` gRPC server auth + interceptor | Test `bufconn` | 1 j |
| M1-07 | `[x]` Gateway: chi router, config, CORS, request-id, error mapping | Test mapping error | 1 j |
| M1-08 | `[x]` JWT middleware + klien gRPC per service (connection reuse) | Token invalid → 401, role kurang → 403 | 1 j |
| M1-09 | `[x]` Handler REST auth + `/me` + rate limit login | Sesuai `API_SPEC.md` §2 | 1 j |
| M1-10 | `[x]` Scaffold frontend: Vite, TS, Tailwind, router, QueryClient | `npm run dev` tampil | 1 j |
| M1-11 | `[x]` API client + auth context + auto refresh 401 | Refresh hanya sekali, tidak loop | 1.5 j |
| M1-12 | `[x]` Halaman login & register + validasi form | Bisa daftar & masuk | 1.5 j |
| M1-13 | `[x]` Guard `/admin` + layout dasar admin | Customer ditolak, admin masuk | 0.5 j |

**Gate M1:** register → login → refresh → logout berjalan; `/admin` terlindungi; test & CI hijau. ✅

**Catatan penyimpangan M1:**
- **`sqlc` ditunda** (M1-03): repo auth memakai `pgx` langsung agar M1 cepat jalan; `sqlc` akan
  diperkenalkan saat katalog (M2) bila pola query sudah stabil.
- **Test repo memakai Postgres dev (DSN), belum `testcontainers`**: cukup untuk lokal & CI saat ini;
  migrasi ke `testcontainers-go` direncanakan pada M3 saat CI butuh DB terisolasi.
- **shadcn/ui ditunda**: komponen UI ditulis manual di `src/components/ui` (Tailwind 4). shadcn bisa
  ditambahkan tanpa mengubah struktur.
- **buf lint**: pesan bersama `User`/`AuthResponse` memakai pengecualian `RPC_REQUEST_RESPONSE_UNIQUE`,
  `RPC_REQUEST_STANDARD_NAME`, `RPC_RESPONSE_STANDARD_NAME` di `buf.yaml`.
- **Frontend memakai proxy Vite** (`/api` → gateway) agar cookie refresh same-origin saat dev.
- **Bug ditemukan & diperbaiki saat verifikasi E2E**: `RotateRefreshToken` mengisi `user_id` dari
  `newToken.UserID` (kosong) alih-alih dari token lama; pencarian identifier email memakai `citext`
  tanpa cast sehingga case-sensitive saat parameter di-infer `text`. Keduanya ditutup test.

---

## M2 — Katalog

| ID | Task | Acceptance | Est |
|---|---|---|---|
| M2-01 | `[x]` Proto `catalog.v1` (termasuk `CheckStock`/`ReserveStock`/`ReleaseStock`) | `buf lint` bersih | 0.5 j |
| M2-02 | `[x]` Migrasi `saybu_catalog`: `categories`, `products`, `product_images`, `stock_reservations` | Naik/turun bersih | 1 j |
| M2-03 | `[x]` Query katalog + repo (pgx langsung; `sqlc` ditunda) | Test repo integrasi lulus | 1 j |
| M2-04 | `[x]` Domain katalog: stok, reservasi, aturan harga, slug unik | Unit test | 1 j |
| M2-05 | `[x]` Usecase produk (CRUD, set/adjust stok) + validasi | Test usecase | 1.5 j |
| M2-06 | `[x]` gRPC server catalog | Test `bufconn` | 1 j |
| M2-07 | `[x]` Gateway: endpoint publik kategori & produk + filter/pagination | Sesuai `API_SPEC.md` §3 | 1 j |
| M2-08 | `[x]` Gateway: endpoint admin produk + upload gambar (validasi tipe/ukuran) | Upload > 5 MB ditolak | 1.5 j |
| M2-09 | `[x]` Seed: 2 kategori + 12 produk + 1 admin (zona ditunda ke M3) | `make seed` idempoten | 1 j |
| M2-10 | `[x]` Storefront: home (kategori + produk unggulan) | Mobile rapi 360 px | 1 j |
| M2-11 | `[x]` Storefront: daftar produk + filter kategori + pencarian | Filter & cari bekerja | 1.5 j |
| M2-12 | `[x]` Storefront: detail produk (galeri, harga, stok, pilih jumlah) | Stok 0 tidak bisa ditambah | 1.5 j |
| M2-13 | `[x]` Cart store (Zustand + persist) + badge & halaman keranjang | Bertahan setelah refresh | 1 j |
| M2-14 | `[x]` Admin: tabel produk + form create/edit + upload foto + ubah stok | CRUD penuh dari UI | 2 j |
| M2-15 | `[x]` Test frontend (Vitest) untuk cart & format uang | `npm test` hijau | 1 j |

**Gate M2:** produk tampil & bisa dicari, admin CRUD + upload jalan, stok tercermin di storefront. ✅

**Catatan penyimpangan M2:**
- **`sqlc` tetap ditunda** (M2-03): repo katalog memakai `pgx` langsung seperti M1. Alasan: `sqlc`
  belum terpasang dan pola query (`ILIKE`, filter dinamis, upsert gambar) lebih mudah ditulis manual.
  Rencana: tinjau ulang sebelum M4 bila jumlah query bertambah banyak.
- **Seed zona ongkir ditunda ke M3** (M2-09): tabel zona baru ada saat M3, jadi seed M2 hanya
  mengisi kategori, produk, dan admin. Seed idempoten (dijalankan dua kali tidak menduplikasi).
- **Tidak ada CRUD kategori** di M2: kategori hanya dari seed (sesuai keputusan scope). Endpoint
  `GET /categories` publik tetap ada.
- **`GET /admin/products` ditambahkan** (tidak ada di `API_SPEC.md` awal) agar admin dapat melihat
  produk nonaktif. Bersifat aditif dan sudah didokumentasikan di `API_SPEC.md` §7.6.
- **Model satuan**: `kg | ikat | pcs | buah | paket`. Untuk `kg`, stok & qty disimpan dalam **gram**
  (satuan terkecil); harga tetap per kg. Varian produk (mis. ukuran) ditunda.
- **Penyimpanan gambar**: disk lokal (`UPLOAD_DIR`, default `./data/uploads`) dengan URL publik
  `/uploads/...`; gateway melayani file statis dan menolak daftar direktori. Object storage/CDN
  ditunda ke M7.
- **Proxy Vite ditambah `/uploads`** selain `/api` agar gambar produk tampil saat dev.
- **Perbaikan bug**: `defer pool.Close()` pada test repo integrasi berjalan lebih dulu daripada
  `t.Cleanup` sehingga baris uji tidak pernah terhapus; diganti `t.Cleanup(pool.Close)` (auth & katalog).
- **`typecheck` frontend**: skrip diubah dari `tsc --noEmit` (no-op karena `tsconfig.json` bergaya
  solusi) menjadi `tsc -b` agar benar-benar memeriksa tipe.

---

## M3 — Ongkir, Checkout, Order, Kafka

| ID | Task | Acceptance | Est |
|---|---|---|---|
| M3-01 | `[x]` Service `shipping`: proto, migrasi `zones`, repo, domain kuotasi | Unit + repo test | 1.5 j |
| M3-02 | `[x]` Gateway: zona publik + quote + CRUD zona admin | Sesuai `API_SPEC.md` §4 | 1 j |
| M3-03 | `[x]` Proto `order.v1` + migrasi `orders`, `order_items`, `order_status_history`, `idempotency_keys`, `outbox` | Naik/turun bersih | 1.5 j |
| M3-04 | `[x]` Domain order: state machine + validasi transisi | Test semua transisi legal/ilegal | 1.5 j |
| M3-05 | `[x]` Usecase checkout: quote ongkir → cek stok → reserve → simpan order → outbox | Test kompensasi saat gagal simpan | 2 j |
| M3-06 | `[x]` Idempotensi checkout (`Idempotency-Key`, hash body) | Double submit → satu order | 1 j |
| M3-07 | `[x]` `pkg/outbox` relay di order + producer Kafka | Event muncul di Kafka UI | 1.5 j |
| M3-08 | `[x]` Gateway: `POST /orders`, `GET /orders/{code}`, cancel | Sesuai `API_SPEC.md` §5 | 1.5 j |
| M3-09 | `[x]` UI checkout: data pembeli, pilih antar/ambil, zona, metode bayar, ringkasan | Total = subtotal + ongkir | 2 j |
| M3-10 | `[x]` UI halaman sukses + instruksi bayar + kode pesanan | Kode tampil & bisa disalin | 1 j |
| M3-11 | `[x]` UI lacak pesanan (kode + no. HP) | Status & riwayat tampil | 1 j |
| M3-12 | `[x]` Snapshot item & alamat di order | Ubah harga produk tidak mengubah order lama | 0.5 j |
| M3-13 | `[x]` Validasi server: harga, stok, step qty, min qty, zona | Test kasus negatif | 1.5 j |
| M3-14 | `[x]` Integration test alur checkout dengan Kafka container | Lulus di CI | 2 j |
| M3-15 | `[x]` Consumer `order.events.v1` di catalog (release stok) | Batal → stok kembali | 1 j |
| M3-16 | `[x]` Job expire order 24 jam → `order.expired` | Order lama otomatis expired | 1 j |

**Gate M3:** checkout end-to-end membuat order + event Kafka; stok ter-reserve; batal melepas stok; tidak ada oversell.

**Catatan penyimpangan M3 (sejauh ini):**
- **Zona ongkir punya `UNIQUE(name)`** agar seed bisa idempoten (`INSERT ... ON CONFLICT (name) DO NOTHING`)
  dan admin tidak membuat zona duplikat. Zona adalah master data sederhana, bukan entitas bisnis.
- **`DELETE` zona = hard delete**, bukan soft delete. Aman karena order menyimpan **snapshot** nama & tarif
  ongkir, sehingga menghapus zona tidak mengubah pesanan lama. Untuk sementara, admin bisa menonaktifkan
  (`is_active=false`) bila ingin menyembunyikan tanpa menghapus.
- **`GET /shipping/quote` dengan zona tidak dikenal/invalid mengembalikan `422`** (sesuai `API_SPEC.md` §4.2),
  dipetakan dari gRPC `FailedPrecondition`. UUID tidak valid diperlakukan sebagai "tidak ditemukan" (bukan 500).
- **Admin `PATCH`/`DELETE` zona tidak dikenal mengembalikan `404`.**
- **Info rekening transfer statis dari `.env`** (`BANK_NAME`, `BANK_ACCOUNT`, `BANK_HOLDER`) — belum ada tabel
  rekening; nilai harus di-quote karena mengandung `&` (mis. `"Saybu Buah & Sayur"`).
- **Seed 4 zona contoh** (`Kec. Sukamaju`, `Kec. Cibaduyut`, `Kec. Dayeuhkolot`, `Ambil di tempat`) — admin dapat
  mengubah/menambah lewat UI.

**Catatan penyimpangan M3 (tahap 2–6):**
- **`order.v1` hanya RPC M3** (`CreateOrder`, `GetOrderByCode`, `ListMyOrders`, `CancelOrder`). Admin
  `ListOrders`/`UpdateStatus`/`GetTodayStats` (`API_SPEC.md` §8.4) ditunda ke M5. RPC internal `ExpireDueOrders`
  tidak dibuat; job expire berjalan lewat ticker internal di service order.
- **Kode pesanan harian berurutan** `SB-YYYYMMDD-NNNN`, nomor urut di tabel `order_code_counters` dengan
  `INSERT ... ON CONFLICT ... RETURNING` (aman dari balapan). Tanggal memakai zona `Asia/Jakarta`.
- **Model stok**: `ReserveStock` langsung mengurangi `products.stock` dan mencatat reservasi `RESERVED`.
  `payment.confirmed` (M4) akan menandai `CONFIRMED`; batal/kedaluwarsa → `ReleaseStock` (status `RELEASED`).
- **Saga dikelola service order**: `shipping.Quote` → `catalog.CheckStock` → `catalog.ReserveStock` →
  `INSERT` order/items/history + outbox `order.created`. Bila `INSERT` gagal setelah reservasi,
  dikompensasi dengan `catalog.ReleaseStock`.
- **Idempotensi**: header `Idempotency-Key` wajib (400 bila kosong); hash body = SHA-256 input ternormalisasi
  (tanpa `user_id`). Replay dengan kunci & body sama → order yang sama; kunci sama body beda → 422.
- **Pemetaan error**: stok kurang → `409`; transisi status ilegal / cancel tidak diizinkan → `409`;
  zona tidak dikenal → `422` (`FailedPrecondition` + detail `zone_id`); `Idempotency-Key` body beda → `422`.
- **Konsumer `order.events.v1` di catalog** memakai group `catalog-svc`; `order.cancelled`/`order.expired` →
  `ReleaseStockOnce` (idempotent lewat `processed_events` dalam transaksi yang sama). Tipe lain diabaikan.
- **Job expire** berjalan tiap `ORDER_EXPIRY_SWEEP` (default `1m`), memakai `FOR UPDATE SKIP LOCKED`,
  menandai `PENDING_PAYMENT` yang lewat `expires_at` menjadi `EXPIRED` + outbox `order.expired`.
  `ORDER_EXPIRY` (default `24h`) menentukan tenggat.
- **Test integrasi Kafka** memakai broker dev (`KAFKA_BROKERS`) + DB dev, bukan testcontainers (konsisten
  dengan penyimpangan M1/M2). Lihat `services/catalog/internal/service/consumer_integration_test.go`.

**Catatan penyimpangan M3 (tahap 7 — frontend):**
- **Rute baru**: `/checkout` (checkout), `/pesanan` (lacak), `/pesanan/:code` (detail + halaman sukses).
  Keranjang → tombol **Checkout** mengarah ke `/checkout`; navbar menambah tautan **Lacak**.
- **Instruksi transfer statis dari env frontend** (`VITE_BANK_NAME`, `VITE_BANK_ACCOUNT`, `VITE_BANK_HOLDER`,
  lihat `frontend/.env.example`), selaras dengan `BANK_*` backend. Endpoint `GET /orders/{code}/payment`
  (M4) belum ada, jadi halaman sukses membaca env frontend sampai M4.
- **Idempotency-Key dibuat di klien** (`crypto.randomUUID`, fallback untuk lingkungan lama) dan **stabil selama
  isi pesanan tidak berubah** — klik ganda/retry jaringan memakai kunci sama sehingga tidak membuat pesanan ganda.
  Kunci diperbarui otomatis bila isi form/keranjang berubah (server menolak body beda dengan kunci sama → 422).
- **Ongkir dihitung ulang di klien hanya untuk tampilan**; server tetap menghitung ulang saat pesanan dibuat.
  Kuotasi diambil dari `GET /shipping/quote` saat zona dipilih.
- **Data pembeli diisi otomatis** dari profil bila pengguna login (tanpa menimpa isian manual).
- **Riwayat pesanan akun** (`GET /me/orders`) belum ditampilkan di UI; tetap di M5 (M5-01). Halaman
  `/pesanan/:code` hanya melacak satu pesanan via kode + HP (guest) atau login (pemilik).
- **Halaman sukses** memakai `?baru=1` untuk banner sukses dan menyertakan `phone` di URL agar guest dapat
  memuat ulang halaman detail (mekanisme sama dengan `API_SPEC.md` §5.2).
- **`lib/api.ts`** kini menerima header kustom (`RequestOptions.headers`) untuk mengirim `Idempotency-Key`.

---

## M4 — Pembayaran & Notifikasi

| ID | Task | Acceptance | Est |
|---|---|---|---|
| M4-01 | `[x]` Proto `payment.v1` + migrasi `saybu_payments` (`payments`, `payment_proofs`, `processed_events`) | Naik/turun bersih | 1 j |
| M4-02 | `[x]` Usecase buat payment saat `order.created` + status awal | Consumer idempoten | 1.5 j |
| M4-03 | `[x]` Upload bukti (validasi tipe/ukuran, simpan ke volume, URL) | File tersimpan & bisa diakses | 1.5 j |
| M4-04 | `[x]` Verifikasi admin (approve/reject) + event | Event `payment.confirmed` terbit | 1 j |
| M4-05 | `[x]` Consumer `payment.events.v1` di order (ubah status) | Status order ikut berubah | 1 j |
| M4-06 | `[x]` Consumer `order.events.v1` di payment (batal/expired) | Payment ditandai batal | 0.5 j |
| M4-07 | `[x]` Service notification: consumer + klien Telegram + retry + DLQ | Pesan masuk Telegram | 1.5 j |
| M4-08 | `[x]` Template pesan: order baru, bukti masuk, terverifikasi, ditolak, batal | Format rapi & informatif | 1 j |
| M4-09 | `[x]` UI instruksi transfer + form upload bukti | Guest bisa upload dengan kode+HP | 1.5 j |
| M4-10 | `[x]` UI admin: daftar pembayaran menunggu + preview bukti + approve/reject | Verifikasi dari HP | 1.5 j |
| M4-11 | `[x]` UI status pembayaran di halaman lacak | Status jelas & real-time-ish | 0.5 j |
| M4-12 | `[x]` Idempotensi consumer (`processed_events`) di semua consumer | Kirim event duplikat → efek sekali | 1 j |
| M4-13 | `[x]` DLQ + perintah replay manual | Pesan gagal bisa diproses ulang | 1 j |
| M4-14 | `[x]` Test integrasi pembayaran penuh | Lulus di CI | 2 j |

**Gate M4:** transfer manual end-to-end (upload → verifikasi → order PAID) + Telegram berfungsi, duplikasi aman.

### Catatan implementasi M4

- **Penyimpangan §8.5:** `UploadProof` bukan RPC `stream` ber-chunk seperti di `API_SPEC.md` §8.5, melainkan unary —
  gateway yang menyimpan berkas ke disk (pola sama dengan unggah gambar katalog M2) lalu memanggil
  `UploadProof(code, phone, user_id, proof_url, size_bytes, content_type)`. Alasan: satu jalur unggah,
  rate limit, dan validasi tipe di gateway; service pembayaran tidak menyimpan berkas. `API_SPEC.md` §8.5 diperbarui.
- **`user_id` pada pembayaran:** tabel `payments` menyimpan `user_id` (dari `order.created`) agar pemilik akun bisa
  membaca status & mengunggah bukti tanpa nomor HP. Field ini aditif pada payload `order.created` (§9.1).
- **Status pembayaran:** `UNPAID | WAITING_VERIFICATION | PAID | REJECTED | CANCELLED | COD_PENDING | COD_PAID`.
  Awal: TRANSFER → `UNPAID`, COD → `COD_PENDING`.
- **Transisi order dari event pembayaran** (idempoten via `processed_events`, lalu memancarkan `order.status_changed`):
  `payment.proof_uploaded` → `PENDING_PAYMENT → WAITING_VERIFICATION`; `payment.confirmed` → `WAITING_VERIFICATION → PAID`;
  `payment.rejected` → `WAITING_VERIFICATION → PENDING_PAYMENT`.
- **Katalog** mengonsumsi `payment.confirmed` untuk menandai reservasi stok `CONFIRMED`.
- **Notification** memakai consumer group `notification-svc` pada `order.events.v1` + `payment.events.v1`;
  dedup persisten di `saybu_notifications.processed_events` (fallback in-memory bila DSN kosong).
  Bila `TELEGRAM_BOT_TOKEN`/`TELEGRAM_ADMIN_CHAT_ID` kosong, pengiriman dilewati & dicatat di log.
- **Frontend:** modul `features/payment` (`types`, `api`, `ProofUploadForm`, `PaymentPanel`) + halaman admin
  `/admin/pembayaran`. Instruksi transfer & unggah bukti dilepas dari `OrderDetailView` ke `PaymentPanel`
  (rekening dari API, fallback `VITE_BANK_*`). `CopyButton` dipindah ke `components/ui`.
- **Bukti transfer** disimpan di prefix `/uploads` yang sama dengan gambar produk (URL UUID tak tertebak, tanpa
  listing direktori) — batasan yang diketahui sampai object storage di M7.

---

## M5 — Kelengkapan

| ID | Task | Acceptance | Est |
|---|---|---|---|
| M5-01 | `[x]` Riwayat pesanan untuk akun (`/me/orders`) | Hanya pesanan sendiri | 1 j |
| M5-02 | `[x]` Admin: daftar & detail pesanan + filter status/tanggal/cari | Filter akurat | 1.5 j |
| M5-03 | `[x]` Admin: ubah status dari UI dengan validasi | Transisi ilegal ditolak | 1 j |
| M5-04 | `[x]` Admin: dashboard ringkasan hari ini | Angka sesuai DB | 1 j |
| M5-05 | `[x]` Pembatalan oleh pembeli + konfirmasi | Stok dilepas | 1 j |
| M5-06 | `[x]` COD: tandai lunas saat selesai | Status pembayaran COD benar | 0.5 j |
| M5-07 | `[x]` Notifikasi stok menipis ke admin | Event `stock.low` terkirim sekali per ambang | 1 j |
| M5-08 | `[x]` Halaman 404/500 + error boundary frontend | Tidak blank screen | 0.5 j |
| M5-09 | `[x]` Empty states & loading skeleton | UX rapi | 0.5 j |
| M5-10 | `[x]` SEO dasar: title, meta, OG image, sitemap | Terbaca crawler | 0.5 j |
| M5-11 | `[x]` Audit log admin sederhana | Perubahan tercatat | 1 j |
| M5-12 | `[ ]` Uji usability panel dengan orangtua | Ada catatan perbaikan | 1 j |

**Catatan M5:**
- **Backend** (commit `b364eaf`): RPC admin `order.v1` (`GetOrderByID`, `ListOrders`, `UpdateStatus`,
  `GetTodayStats`), `catalog.ListLowStock`, gateway `/admin/orders` (+detail, ubah status),
  `/admin/stats/today`, audit middleware, payment `order.status_changed` → COD `COD_PAID`, notification
  topik `catalog.events.v1` + pesan `stock.low`.
- **Definisi statistik:** `orders_count` = pesanan dibuat hari ini (Asia/Jakarta); `revenue_paid` = jumlah
  `total` pesanan hari ini berstatus `PAID`/`PREPARING`/`SHIPPED`/`READY_PICKUP`/`COMPLETED`;
  `awaiting_verification` = jumlah pesanan berstatus `WAITING_VERIFICATION` saat ini (backlog, bukan
  hanya hari ini).
- **`low_stock`** digabung di gateway dari `catalog.ListLowStock` (satu DB per service dijaga); bila
  panggilan katalog gagal, daftar kosong + log peringatan (ringkasan tetap tampil).
- **Audit log** persisten di tabel `audit_logs` (database `saybu_auth`) lewat RPC
  `auth.v1.AuthService.RecordAudit` dari gateway; log terstruktur tetap dipertahankan sebagai
  fallback. `request_id` menjadi kunci idempotensi. Perubahan status pesanan tetap tercatat di
  `order_status_history`.
- **`stock.low`** ditulis lewat outbox pada transaksi `ReserveStock` saat checkout menembus ambang
  (sekali per pelanggaran ambang), juga tetap dipancarkan dari jalur update stok produk.
- **SEO/OG:** `og-image.svg` & `favicon.svg` adalah placeholder SVG; ganti OG image raster + domain
  absolut saat deploy (M7).

**Gate M5:** seluruh alur bisnis v1 lengkap dan bisa dipakai sehari-hari.

---

## M6 — Observability & hardening

| ID | Task | Acceptance | Est |
|---|---|---|---|
| M6-01 | `[x]` Metrik Prometheus semua service | `/metrics` berisi metrik kunci | 1 j |
| M6-02 | `[x]` Dashboard Grafana (request rate, latency, error, consumer lag, outbox lag) | Dashboard bisa dipakai | 1.5 j |
| M6-03 | `[x]` Propagasi `request_id` HTTP → gRPC → event → log | Satu request bisa dilacak | 1 j |
| M6-04 | `[x]` Rate limit seluruh endpoint sesuai spec | 429 muncul saat melebihi | 0.5 j |
| M6-05 | `[x]` Security review: CORS, header, secret, upload, SQL injection | Checklist aman | 1 j |
| M6-06 | `[x]` Load test k6 (100 VU katalog + 10 checkout/detik) | p95 < 300 ms lokal | 1 j |
| M6-07 | `[x]` Uji restore backup | Data kembali utuh | 1 j |
| M6-08 | `[x]` Dokumentasi operasional (runbook) | Ada panduan insiden | 1 j |
| M6-09 | `[x]` Alerting Prometheus/Grafana → Telegram (service down, 5xx, Kafka lag, outbox lag) | Rule tampil & contact point valid | 1 j |

**Catatan M6:**
- **M6-09 alerting:** provisioning Grafana di `infra/observability/grafana/provisioning/alerting`
  (4 rule + contact point Telegram + notification policy). Token/chat id dari environment;
  placeholder `disabled` dipakai saat kredensial kosong agar provisioning tetap valid.
  Diverifikasi: 4 rule muncul via `/api/v1/provisioning/alert-rules`, root policy mengarah ke
  `telegram-admin`. Panduan: RUNBOOK §14.
- **M6-03 korelasi:** `kafkax.NewEventCtx(ctx, …)` mengisi `correlation_id` dari `request_id` context;
  semua pembangun event (order/catalog/payment) meneruskan `ctx`. Consumer menyemai `request_id` handler
  dari `correlation_id` (cadangan `event_id`). Diverifikasi end-to-end: `X-Request-Id` HTTP → gRPC →
  `order.created` (`correlation_id` sama) → log gateway & order.
- **M6-01 metrik:** `telemetry.Metrics` menambah `grpc_server_handling_seconds`, `outbox_lag_seconds`
  (+`ObserveOutboxLag`), `kafka_consumer_lag` (+`SetConsumerLag`), `orders_created_total`
  (+`IncOrdersCreated`). `outbox.Relay.SetLagReporter` melaporkan usia event tertua; `kafkax.Consumer`
  menghitung lag via `kadm` (`ReportLagLoop` 15s). Order memakai `SetOrderCreatedHook`.
- **M6-04 rate limit:** middleware `ratelimit.Middleware` + `ReadMethods`/`WriteMethods`/`ByIP`;
  baca `/api/v1` 120/menit per IP, tulis admin 60/menit per user; semua 429 menyertakan `Retry-After`.
  Limiter kini membersihkan bucket kedaluwarsa (`nextSweep`/`Size`) agar memori terbatas.
- **M6-02 observability:** compose profil `observability` (Prometheus v3.1.0, Grafana 11.5.1,
  volume `promdata`/`grafanadata`, `host.docker.internal`), provisioning datasource + dashboard
  (8 panel). Target `make observability`/`observability-down`. Env baru `PROMETHEUS_PORT`,
  `GRAFANA_PORT`, `GRAFANA_USER`, `GRAFANA_PASSWORD`. Diverifikasi: 7 target `up`, dashboard tampil.
- **M6-05 security:** `httpx.SecurityHeaders(hsts)` (nosniff, X-Frame-Options DENY, Referrer-Policy,
  Permissions-Policy, HSTS produksi) di gateway; CORS menambah `Accept-Language`. Review: validasi
  unggah, path traversal, `.env` ter-gitignore, query berparameter. Checklist: `docs/SECURITY.md`.
- **M6-06 load test:** `infra/load/k6-saybu.js` (k6 via Docker, `make load-test`), IP klien acak per
  iterasi. Hasil: katalog p95 ≈ 242 ms (< 300) dan checkout p95 ≈ 449 ms (< 500), gagal katalog 0%.
- **M6-07 backup/restore:** `infra/scripts/{backup.sh,restore.sh}` + `make backup`/`restore` (nama DB
  dari DSN). Diuji: backup 5 DB, rusak data (hapus produk/pesanan/user), restore → jumlah baris & versi
  migrasi (`goose_db_version`) kembali persis.
- **M6-08 runbook:** `docs/RUNBOOK.md` (16 bagian: triase, insiden service/DB/Kafka/lag/outbox/DLQ,
  restore, migrasi, pembayaran, notifikasi, disk, observability, load test, eskalasi).

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

## UI — Penyesuaian reference design (beranda & katalog)

| ID | Status | Task | DoD | Estimasi |
|---|---|---|---|---|
| UI-01 | `[x]` | Bottom nav 5 tab + header ringkas di mobile, ikon SVG inline | Navigasi aktif benar, target sentuh ≥ 44 px | 1 j |
| UI-02 | `[x]` | Hero bergambar + grid "Belanja cepat" di beranda | Tampil di mobile & desktop, fallback gambar aman | 1 j |
| UI-03 | `[x]` | Kartu produk baru: badge kategori, `PriceTag` aksen oranye, tombol + | Harga tetap dari server, test RTL hijau | 1 j |
| UI-04 | `[x]` | Filter katalog sticky + grid produk 2/3 kolom | Cari & filter kategori bekerja, test RTL hijau | 0.5 j |
| UI-05 | `[x]` | Aset WebP teroptimasi (<100 KB/foto) dari Unsplash + catatan sumber | Semua file WebP valid, `SOURCES.md` ada | 0.5 j |
| UI-06 | `[x]` | Screenshot hasil ke `ui-app/` (mobile & desktop) via Playwright | 6 PNG terbentuk dari data contoh | 0.5 j |
| UI-07 | `[x]` | Rework seluruh frontend ke shadcn/ui v4 + Tailwind, tema netral | Adapter jalur lama tetap kompatibel, 96 test hijau, tanpa palet brand/accent | 2 j |

Verifikasi (2026-11-22): `npm test` (21 file / 96 test), `npm run lint` (0 issue), `npm run typecheck`, `npm run build` hijau.
Screenshot memakai fixture API (`page.route`) agar tidak bergantung backend. Kontrak API tidak berubah.

Catatan UI-07: komponen kanonik shadcn berada di `src/components/ui/shadcn/` (repo case-insensitive,
jadi `Button.tsx` dan `button.tsx` tidak bisa satu folder). File `Button.tsx`, `Card.tsx`,
`Input.tsx`, `Select.tsx`, `Feedback.tsx`, `CopyButton.tsx`, `BottomNav.tsx` menjadi adapter
yang memetakan API lama ke primitive shadcn. `Select` sengaja native agar `selectOptions`
pada test RTL tetap bekerja. Semua warna memakai token `--background/foreground/primary/…`;
status memakai `muted`, `secondary`, dan `destructive`.

Lanjutan 2026-11-22: seluruh 12 produk seed kini punya entri `product_images` (foto spesifik
untuk pisang/apel/bayam/tomat, default kategori untuk sisanya). `ProductImage` memakai
`/images/produk-default.webp` bila `src` kosong atau gagal dimuat. Seed hanya menambah gambar
yang belum ada, tidak menimpa unggahan admin.

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
| 2026-11-12 | **M1 selesai.** Auth (register/login/refresh rotasi+reuse/logout), gateway REST + JWT + rate limit, frontend auth (login/register/guard admin). Verifikasi E2E manual via curl lulus; unit + integrasi (repo) hijau |
| 2026-11-12 | Penyimpangan M1: sqlc ditunda (pgx langsung), test repo pakai Postgres dev (belum testcontainers), shadcn ditunda |
| 2026-11-12 | Bug diperbaiki saat verifikasi: `user_id` kosong pada rotasi refresh token; pencarian email `citext` case-sensitive. Keduanya kini ada test |
| 2026-11-13 | **M2 selesai.** Katalog: proto + migrasi (kategori/produk/gambar/reservasi stok/outbox), domain+repo+usecase+gRPC, gateway publik & admin, upload gambar disk, seed idempoten. Frontend: home, daftar/detail produk, keranjang (Zustand persist), admin CRUD produk. Verifikasi E2E manual via curl lulus; unit + integrasi (repo) + Vitest hijau |
| 2026-11-13 | Penyimpangan M2: sqlc tetap ditunda (pgx langsung), seed zona ditunda ke M3, tanpa CRUD kategori, `GET /admin/products` ditambahkan, varian produk ditunda |
| 2026-11-13 | Bug diperbaiki: cleanup test repo integrasi (`defer pool.Close()` → `t.Cleanup`); skrip `typecheck` frontend `tsc --noEmit` (no-op) → `tsc -b` |
| 2026-11-14 | **M3 Tahap 1 selesai (M3-01, M3-02).** Service `shipping`: proto `shipping.v1` (`ListZones`/`Quote`/`UpsertZone`/`DeleteZone`), migrasi `zones` (naik/turun bersih), domain kuotasi (gratis bila `subtotal ≥ free_min_subtotal`), repo pgx, gRPC + admin HTTP. Gateway: zona publik, quote, CRUD zona admin. Seed 4 zona contoh idempoten. Verifikasi E2E manual via curl (kuotasi, zona nonaktif tersembunyi dari publik, CRUD admin, 401/404/409/422) lulus; unit + integrasi (repo) hijau |
| 2026-11-14 | Penyimpangan M3 (tahap 1): zona `UNIQUE(name)` untuk seed idempoten, DELETE = hard delete (order snapshot nama/tarif), quote zona tak dikenal → 422, info rekening statis dari `.env` |
| 2026-11-14 | **M3 Tahap 2–6 selesai (M3-03..M3-08, M3-12..M3-16).** Service `order` baru: proto `order.v1`, migrasi `orders`/`order_items`/`order_status_history`/`idempotency_keys`/`order_code_counters`, state machine, saga checkout (quote → cek stok → reserve → simpan + outbox `order.created`), idempotensi, job expire + outbox `order.expired`. Gateway: `POST /orders` (rate limit 10/mnt), `GET /orders/{code}`, `POST /orders/{code}/cancel`, `GET /me/orders` + `authmw.Optional`. Konsumer `order.events.v1` di catalog (`catalog-svc`) melepas stok idempoten. Verifikasi E2E manual via curl lulus (201, replay 201, body beda 422, tanpa header 400, stok kurang 409, zona tak dikenal 422, lacak 200/404, cancel → stok kembali & reservasi `RELEASED`, `/me/orders` 200/401, expire otomatis → stok kembali); unit + integrasi (repo + Kafka) hijau |
| 2026-11-14 | Penyimpangan M3 (tahap 2–6): `order.v1` hanya RPC M3 (admin §8.4 → M5), tanpa RPC `ExpireDueOrders` (ticker internal), test integrasi Kafka pakai broker dev (bukan testcontainers) |
| 2026-11-15 | **M3 Tahap 7 selesai (M3-09..M3-11) — frontend checkout.** Rute baru `/checkout`, `/pesanan`, `/pesanan/:code`. Form checkout (data pembeli, antar/ambil, zona + kuotasi, alamat, metode bayar, ringkasan total), halaman sukses dengan instruksi transfer statis (`VITE_BANK_*`) + kode pesanan yang bisa disalin, dan halaman lacak (kode + HP) dengan status & riwayat. Idempotency-Key dibuat klien dan stabil selama isi pesanan tidak berubah; `lib/api.ts` mendukung header kustom. Verifikasi: `eslint`, `tsc -b`, `vitest` (59 test) dan `vite build` hijau. Uji E2E manual tertunda karena infra Docker (Postgres/Kafka) mati di lingkungan ini |
| 2026-11-15 | Penyimpangan M3 (tahap 7): instruksi transfer dari env frontend (`VITE_BANK_*`) sampai endpoint pembayaran M4 ada; riwayat pesanan akun (`/me/orders`) tetap M5; halaman detail guest menyertakan `phone` di URL |
| 2026-11-16 | **M4 backend selesai (M4-01..M4-08, M4-12..M4-14).** Service `payment` baru: proto `payment.v1`, migrasi (`payments`+`user_id`, `payment_proofs`, `outbox`, `processed_events`), domain (TRANSFER/COD, 7 status, aturan upload & verifikasi), repo pgx, usecase (status, upload bukti, verifikasi, konsumsi `order.events.v1`), gRPC + cmd (relay outbox, consumer, readiness). Integrasi: order konsumsi `payment.events.v1` (`TransitionFromPayment` idempoten + `order.status_changed`), catalog konsumsi `payment.confirmed` → reservasi `CONFIRMED`, gateway (`GET /orders/{code}/payment`, `POST /orders/{code}/payment-proof` rate limit 5/mnt, `GET /admin/payments`, `POST /admin/payments/{id}/verify`), notification Telegram (consumer group `notification-svc`, dedup in-memory, dilewati bila token kosong). `pkg`: `storage.SaveWith` + `AllowedProofExt`, `kafkax.Consumer.Ping`. `make fmt && make lint && make test` hijau |
| 2026-11-16 | Penyimpangan M4: `UploadProof` unary (bukan stream chunked §8.5) — gateway simpan berkas lalu kirim URL; `user_id` ditambahkan ke tabel `payments` & payload `order.created`; bukti transfer di prefix `/uploads` yang sama (URL UUID, tanpa listing) sampai object storage M7 |
| 2026-11-16 | **M4 frontend selesai (M4-09..M4-11).** Modul `features/payment` (`types`, `api`, `ProofUploadForm`, `PaymentPanel`) + halaman admin `/admin/pembayaran` (filter status, tabel, setujui, tolak dengan alasan, paginasi). `OrderDetailPage` merender `PaymentPanel`; instruksi transfer/COD & unggah bukti dilepas dari `OrderDetailView`. Menu "Pembayaran" + kartu "Menunggu verifikasi" di dashboard. `lib/api` mendukung field form tambahan (mis. `phone`); `CopyButton` ke `components/ui`. Verifikasi: `eslint`, `tsc -b`, `vitest` (77 test), `vite build` hijau. Uji E2E manual & integrasi DB/Kafka tertunda karena infra Docker mati di lingkungan ini |
| 2026-11-17 | **M5 backend selesai (M5-02..M5-07, M5-11).** `order.v1`: RPC `GetOrderByID`/`ListOrders`/`UpdateStatus`/`GetTodayStats` + domain filter & statistik. `catalog`: `ListLowStock`, dan `ReserveStock` memancarkan `stock.low` lewat outbox saat checkout menembus ambang. Gateway: `/admin/orders` (+`{id}`, `{id}/status`), `/admin/stats/today` (menggabungkan `low_stock` dari katalog), middleware audit mutasi admin. `payment`: konsumsi `order.status_changed` → COD `COD_PAID` saat `COMPLETED`. `notification`: topik `catalog.events.v1` + pesan `stock.low`. `make fmt && make lint && make test` hijau |
| 2026-11-17 | **M5 frontend selesai (M5-01..M5-05, M5-08..M5-10).** Modul `features/admin` (`api`/`types`, `AdminOrdersPage`, `AdminOrderDetailPage`, `AdminDashboardPage` dengan `low_stock`), riwayat pesanan di `AccountPage`, `ErrorBoundary` + `/500` + 404, `usePageTitle` di seluruh halaman, `robots.txt`/`sitemap.xml`/OG & favicon SVG. `allowedTransitions` di `features/checkout/status`. Verifikasi: `eslint`, `tsc -b`, `vitest` (88 test), `vite build` hijau |
| 2026-11-17 | Penyimpangan M5: audit log masih log terstruktur gateway (tabel `audit_logs` ditunda); `low_stock` digabung di gateway (satu DB per service dijaga); OG image & favicon berupa SVG placeholder (raster + domain absolut saat M7). Uji E2E manual & integrasi DB/Kafka masih tertunda karena infra Docker mati di lingkungan ini |
| 2026-11-17 | **Verifikasi M0–M5 dengan infra Docker hidup.** Docker Desktop dihidupkan (WSL integration) → `make up` + `make migrate` (5 DB, 8 topic) → `make test` hijau; `make test-integration` hijau (7 paket: repo auth/catalog/shipping/order/payment + consumer catalog + Kafka); E2E manual via gateway **72/72 lulus** (katalog, ongkir, checkout+idempotensi, bukti transfer, verifikasi admin, transisi status & 409, ringkasan harian + `low_stock`, riwayat akun, pembatalan pembeli, COD→`COD_PAID`, event `stock.low`, audit log); `vitest` 88 + `eslint`/`tsc -b` + `make build` hijau |
| 2026-11-17 | Bug diperbaiki saat verifikasi: `payment.repo.AttachProof` menyisipkan baris bukti sebelum memvalidasi status, sehingga unggahan ulang saat `WAITING_VERIFICATION` (atau id bukti duplikat) gagal dengan error primary key alih-alih `ErrProofNotAllowed`. Kini validasi status (UPDATE terkunci) dijalankan lebih dulu; test integrasi repo pembayaran hijau |
| 2026-11-18 | **M6 selesai (M6-01..M6-08).** Observability & hardening: propagasi `correlation_id` HTTP→gRPC→event→log; metrik `grpc_server_handling_seconds`/`kafka_consumer_lag`/`outbox_lag_seconds`/`orders_created_total` (lag via `kadm`); compose profil `observability` (Prometheus + Grafana, dashboard 8 panel); rate limit baca 120/mnt per IP + tulis admin 60/mnt per user dengan eviction bucket; header keamanan + `docs/SECURITY.md`; load test k6 (`make load-test`); backup/restore (`make backup`/`restore`) diuji utuh; `docs/RUNBOOK.md`. `make fmt && make lint && make test` hijau |
| 2026-11-18 | Penyimpangan M6: rate limiter in-memory per instance (Redis ditunda ke M7 bila gateway di-scale); CSP belum dipasang (via Caddy M7); backup masih manual (otomatisasi + object storage di M7). Hasil load test: katalog p95 ≈ 242 ms, checkout p95 ≈ 449 ms (ambang 300/500 ms) |
| 2026-11-18 | UI-01..UI-06 selesai. Beranda & katalog diselaraskan dengan reference design (`refernce-design/`): bottom nav mobile + header ringkas, hero bergambar, grid kategori, kartu produk dengan aksen oranye, filter katalog sticky. Aset WebP lokal dari Unsplash (target <100 KB/file, sumber di `frontend/public/images/SOURCES.md`). Playwright dipasang untuk screenshot; hasil ada di `ui-app/` (6 PNG: beranda/produk × mobile viewport, mobile penuh, desktop). Screenshot memakai fixture API sehingga tidak perlu backend |
