# AGENTS.md — Panduan untuk AI Agent & Kontributor

Dokumen ini adalah **instruksi kerja utama** untuk siapa pun (manusia atau AI) yang mengubah repo ini.
Baca ini dulu, lalu baca dokumen rujukan di `docs/`.

---

## 1. Apa proyek ini

**Saybu** — toko online buah & sayur untuk usaha keluarga, sekaligus proyek latihan
**microservices Go + Kafka**. Frontend React SPA, backend 7 service Go berkomunikasi gRPC,
event lewat Kafka, database Postgres terpisah per service.

| Dokumen | Isi |
|---|---|
| `docs/PRD.md` | Kebutuhan produk, persona, alur, metrik |
| `docs/ARSITEKTUR.md` | Service, diagram, Kafka, outbox, saga, ADR |
| `docs/TECH_STACK.md` | Versi & library, konvensi, env, Makefile, port |
| `docs/API_SPEC.md` | Kontrak REST, gRPC, dan event Kafka |
| `docs/TASK.md` | Backlog per milestone + Definition of Done |

**Sumber kebenaran:** bila kode dan dokumen berbeda, dokumen menang sampai diperbarui secara sadar.

---

## 2. Status saat ini

- [x] Dokumen perencanaan lengkap
- [x] **M0 Fondasi** — infra, `pkg`, proto, 7 skeleton service, Makefile, CI
- [x] **M1 Auth & Gateway** — auth service (JWT + refresh rotasi), gateway REST, frontend login/register + guard admin
- [x] **M2 Katalog** — catalog service (produk, stok, reservasi, outbox), gateway publik & admin, upload gambar, seed, storefront + admin produk
- [x] **M3 Order** — shipping zones & kuotasi, order service (saga checkout, state machine, idempotensi, job expire), gateway order, konsumer stok, frontend checkout + halaman sukses + lacak pesanan
- [x] **M4 Payment & Notifikasi** — payment service (bukti transfer, verifikasi admin, status), konsumer order & catalog, notifikasi Telegram, frontend panel pembayaran + admin verifikasi
- [x] **M5 Kelengkapan** — riwayat pesanan akun, admin pesanan (daftar/detail/ubah status) & ringkasan harian, pembatalan pembeli, COD lunas saat selesai, notifikasi stok menipis, halaman 404/500 + error boundary, SEO dasar, audit mutasi admin
- [ ] M6 Observability · M7 Deploy

> Saat mulai bekerja, perbarui bagian ini dan centang task terkait di `docs/TASK.md`.

---

## 3. Aturan yang tidak boleh dilanggar

1. **Jangan percaya harga/qty dari client.** Server selalu menghitung ulang harga, ongkir, dan total.
2. **Uang = `int64` rupiah.** Tidak pernah `float`. Stok = `int64` satuan terkecil (gram/pcs).
3. **Satu database per service.** Dilarang query ke DB service lain. Integrasi lewat gRPC/Kafka.
4. **Event hanya lewat outbox.** Jangan publish Kafka langsung di handler tanpa menulis ke tabel `outbox` dalam transaksi yang sama.
5. **Consumer wajib idempotent.** Cek & insert `event_id` ke `processed_events` dalam transaksi yang sama dengan efeknya.
6. **Snapshot di pesanan.** Simpan nama, harga, satuan, alamat, ongkir saat checkout.
7. **Tidak ada secret di repo.** Gunakan `.env` (di-gitignore); perbarui `.env.example`.
8. **Jangan ubah kontrak tanpa memperbarui `API_SPEC.md`** dan test.
9. **Migrasi harus bisa naik & turun** (`goose up` / `goose down`).
10. **Log tidak boleh memuat** password, token, atau isi bukti transfer.

---

## 4. Peta arsitektur

```
Browser → Caddy → Gateway (REST :8081) ─gRPC→ auth :50051
                                          ├─→ catalog :50052
                                          ├─→ shipping :50053
                                          ├─→ order :50054
                                          └─→ payment :50055
Kafka :29092 (host) / :9092 (internal) ←→ semua service
notification :8086 ← consumer Kafka → Telegram
Postgres :5433 (host, 5 DB terpisah) · Kafka UI :8090 · Frontend dev :5173
Admin HTTP (health/metrics): auth :9101 · catalog :9102 · shipping :9103 · order :9104 · payment :9105
```

Detail lengkap: `docs/ARSITEKTUR.md`.

---

## 5. Struktur repo

```
saybu/
├── AGENTS.md
├── README.md
├── Makefile
├── docker-compose.yml
├── .env.example
├── docs/                      # PRD, ARSITEKTUR, TECH_STACK, API_SPEC, TASK
├── backend/
│   ├── go.mod                 # satu module: saybu
│   ├── pkg/                   # library bersama (config, logger, httpx, grpcx, kafkax, outbox, ...)
│   ├── gen/                   # hasil buf generate (di-commit, jangan edit manual)
│   ├── proto/saybu/<svc>/v1/
│   └── services/<svc>/
│       ├── cmd/main.go
│       ├── internal/{config,domain,repo,service,transport}
│       ├── migrations/
│       ├── queries/
│       └── Dockerfile
├── frontend/
│   └── src/{app,features,components,lib,styles}
└── infra/{Caddyfile,postgres,kafka,scripts}
```

---

## 6. Perintah penting

```bash
make help          # daftar target
make up            # Postgres + Kafka + Kafka UI
make down
make logs
make proto         # buf lint + generate (setelah ubah .proto)
make migrate       # migrasi semua DB
make seed          # data contoh
make dev           # semua service Go (paralel)
make dev-frontend  # Vite
make test          # unit test
make test-integration  # test Kafka (infra harus hidup)
make lint
make fmt
make build
```

Menjalankan satu service: `cd backend && go run ./services/order/cmd`.

---

## 7. Alur kerja menambah fitur

### 7.1 Endpoint REST baru

1. Tulis/tambahkan RPC di `backend/proto/saybu/<svc>/v1/`.
2. `make proto`.
3. Implementasi di `internal/transport/grpc/` → panggil `internal/service/` (usecase).
4. Logika bisnis di `internal/domain/` (murni, tanpa infra) + unit test.
5. Akses DB lewat `internal/repo/` (sqlc/pgx) + test.
6. Tambahkan handler REST di `services/gateway` (mapping request/response, auth, error).
7. Perbarui `docs/API_SPEC.md`.
8. Test: domain, usecase, handler (httptest), dan integrasi bila menyentuh DB/Kafka.

### 7.2 Event Kafka baru

1. Tambah tipe di `docs/ARSITEKTUR.md` §5 dan payload di `docs/API_SPEC.md` §9.
2. Producer: dalam transaksi DB, `INSERT INTO outbox (...)` — **jangan** publish langsung.
3. Consumer: buat handler idempotent + daftarkan di consumer group service.
4. Retry 3x → DLQ; jangan menelan error tanpa log.
5. Test dengan container Kafka (testcontainers).

### 7.3 Service baru

1. Salin struktur service yang ada (jangan bikin pola baru).
2. Tambah DB + DSN di `docker-compose.yml`, `.env.example`, `Makefile`.
3. Tambah port di `docs/TECH_STACK.md` §6.
4. Tambah health/ready/metrics + Dockerfile.
5. Tambah ke gateway (klien gRPC) bila perlu diakses browser.
6. Update diagram di `docs/ARSITEKTUR.md`.

### 7.4 Migrasi DB

```bash
cd backend/services/<svc>
goose -dir migrations postgres "$DSN" create add_xxx sql
goose -dir migrations postgres "$DSN" up
goose -dir migrations postgres "$DSN" down
```

---

## 8. Konvensi kode

**Go**
- `gofmt`; `golangci-lint` bila tersedia.
- Error domain dengan `errors.Is`/`errors.As`, bungkus `%w`; map ke gRPC di transport.
- Tidak ada `panic` di jalur request.
- `context` selalu diteruskan; `context.Background()` hanya di `main`/job.
- Nama paket pendek, tanpa stutter (`catalog.Product`, bukan `catalog.CatalogProduct`).
- Test memakai table-driven.

**Frontend**
- TypeScript strict; tidak ada `any` tanpa alasan.
- Semua fetch lewat `lib/api.ts` (penanganan 401 + refresh terpusat).
- Uang & tanggal diformat lewat helper `id-ID` / Asia/Jakarta.
- Komponen admin di `features/admin`, dilindungi `RequireRole`.
- Mobile-first, target sentuh ≥ 44 px.

**Git**
- Commit kecil & fokus. Format: `feat(order): checkout idempoten` / `fix(catalog): stok negatif`.
- Jangan commit `.env`, `data/uploads`, atau `backend/gen` bila diputuskan di-generate saat build.

---

## 9. Testing

| Lapisan | Alat | Wajib |
|---|---|---|
| Domain | `testing` + `testify` | Ya, ≥ 70% |
| Repo | `testcontainers-go` (Postgres) | Ya |
| gRPC | `bufconn` | Ya untuk RPC baru |
| Gateway | `httptest` | Ya untuk endpoint baru |
| Kafka | testcontainers | Ya untuk producer/consumer baru |
| Frontend | Vitest + RTL | Ya untuk cart, form, guard |
| E2E | Playwright (M5+) | Alur checkout |

`make test` harus hijau sebelum task dianggap selesai.

---

## 10. Jangan lakukan ini

- ❌ Menaruh logika bisnis di handler HTTP/gRPC.
- ❌ Query lintas database atau join antar service.
- ❌ Publish Kafka di dalam handler tanpa outbox.
- ❌ Menyimpan harga final dari client.
- ❌ Mengubah proto dengan nomor field yang sudah dipakai (breaking change).
- ❌ Menghapus/menimpa data pengguna tanpa migrasi & backup.
- ❌ Menambah dependency besar tanpa alasan (mis. ORM berat, framework baru).
- ❌ Menyalin seluruh dokumen ini ke README — README cukup ringkas dan menunjuk ke `docs/`.

---

## 11. Saat menyelesaikan pekerjaan

1. `make fmt && make lint && make test`.
2. Uji manual alur yang berubah.
3. Perbarui `docs/TASK.md` (centang + catatan).
4. Perbarui `docs/API_SPEC.md` / `docs/ARSITEKTUR.md` bila kontrak berubah.
5. Tulis ringkasan singkat: apa yang berubah, cara menguji, risiko.

---

## 12. Info lingkungan

- Development: WSL2 Ubuntu, Go 1.26 (Linux), Node 20+, Docker 28 + Compose v2.
  Pastikan `go version` menampilkan `linux/amd64` — Go Windows (`go.exe`) tidak bisa dipakai untuk build Linux.
- Produksi nanti: 1 VPS ≥ 4 GB RAM (Kafka), Caddy untuk TLS.
- Zona waktu UI: Asia/Jakarta. DB menyimpan UTC.
- Bahasa UI & pesan error: Indonesia.
