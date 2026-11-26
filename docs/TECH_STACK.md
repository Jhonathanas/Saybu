# TECH STACK — Saybu

| Field | Value |
|---|---|
| Versi | 1.0 |
| Terakhir diperbarui | 2026-11-11 |

---

## 1. Ringkasan

| Lapisan | Teknologi | Versi target |
|---|---|---|
| Bahasa backend | Go | 1.26 |
| Frontend | React + TypeScript + Vite | React 19, TS 5.x, Vite 7 |
| Database | PostgreSQL | 17 |
| Message broker | Apache Kafka (KRaft) | 3.9 |
| API internal | gRPC + Protocol Buffers (buf) | grpc-go 1.7x, buf 1.4x |
| API publik | REST/JSON | — |
| Container | Docker + Docker Compose | 28 / v2.39 |
| Reverse proxy / TLS | Caddy | 2.10 |
| CI/CD | GitHub Actions | — |
| Notifikasi | Telegram Bot API | — |
| Observability | Prometheus + Grafana (opsional) | 3.x / 11.x |

---

## 2. Backend (Go)

### 2.1 Library inti

| Kebutuhan | Pilihan | Alasan |
|---|---|---|
| Router HTTP | `github.com/go-chi/chi/v5` | Ringan, idiomatik, middleware jelas |
| gRPC | `google.golang.org/grpc` | Standar |
| Protobuf codegen | `buf` + `protoc-gen-go`, `protoc-gen-go-grpc` | Tooling modern, lint & breaking check |
| Kafka | `github.com/twmb/franz-go` (+ `pkg/kadm` untuk lag consumer group) | Pure Go, cepat, idempotent producer, consumer group matang |
| Postgres driver | `github.com/jackc/pgx/v5` | Performa & fitur terbaik |
| Query codegen | `sqlc` | SQL eksplisit, type-safe |
| Migrasi | `github.com/pressly/goose/v3` | Sederhana, bisa embed |
| Validasi | `github.com/go-playground/validator/v10` | Umum, tag jelas |
| JWT | `github.com/golang-jwt/jwt/v5` | Standar |
| Password | `golang.org/x/crypto/argon2` | Argon2id |
| UUID | `github.com/google/uuid` (v7) | Time-ordered, index-friendly |
| Config | `github.com/caarlos0/env/v11` + `.env` | Env-first 12-factor |
| Logging | `log/slog` (stdlib) | JSON handler, tanpa dependensi |
| Metrik | `github.com/prometheus/client_golang` | Standar |
| Testing | `testing`, `testify`, `testcontainers-go`, `bufconn` | Unit, integrasi, gRPC |
| Live reload | `air` | Dev only |

### 2.2 Struktur service (seragam)

```
services/<name>/
├── cmd/main.go
├── internal/
│   ├── config/config.go
│   ├── domain/          # entity + aturan bisnis murni (tanpa import infra)
│   ├── repo/            # pgx / sqlc
│   ├── service/         # usecase, transaksi, outbox
│   └── transport/
│       ├── grpc/        # implementasi proto
│       └── http/        # health, metrics
├── migrations/
├── queries/             # *.sql untuk sqlc
├── sqlc.yaml
└── Dockerfile
```

### 2.3 Shared package (`backend/pkg`)

| Package | Isi |
|---|---|
| `config` | loader env, validasi wajib |
| `logger` | slog JSON + helper context |
| `httpx` | response envelope, error mapping, middleware |
| `grpcx` | interceptor: request-id, logging, recovery, timeout, auth |
| `grpcapp` | runner service gRPC + admin HTTP + graceful shutdown |
| `kafkax` | producer/consumer wrapper, retry, DLQ, event envelope |
| `outbox` | tabel + relay generic |
| `money` | tipe rupiah, format, parse |
| `ids` | UUIDv7, kode pesanan |
| `storage` | interface upload + implementasi disk |
| `telemetry` | request-id, metrik dasar |

> **Satu Go module.** Semua service berada di module `saybu` (`backend/go.mod`), bukan `go.work`.
> Service tetap menjadi binary & container terpisah; ini menyederhanakan build, CI, dan Docker.
> Hasil `buf generate` (`backend/gen`) di-commit agar build tidak membutuhkan buf.

### 2.4 Konvensi Go

- Error domain: `var ErrNotFound = errors.New(...)`, dibungkus `%w`; mapping ke gRPC di transport.
- Tidak ada `panic` di jalur request; recovery interceptor sebagai jaring terakhir.
- Context selalu diteruskan; tidak ada `context.Background()` di handler.
- Semua uang `int64`; stok `int64` satuan terkecil.
- Waktu disimpan UTC (`timestamptz`), dikonversi ke WIB di presentasi.
- Format: `gofmt` (nanti `gofumpt` + `goimports`); lint `golangci-lint` bila tersedia.
- Setiap service wajib: `Dockerfile` multi-stage, `README.md` singkat, `.env.example` var.

---

## 3. Frontend

| Kebutuhan | Pilihan |
|---|---|
| Build tool | Vite 7 |
| Framework | React 19 + TypeScript |
| Routing | React Router 7 |
| Data fetching | TanStack Query v5 |
| State keranjang | Zustand + persist (localStorage) |
| Form & validasi | react-hook-form + zod |
| Styling | Tailwind CSS 4 |
| Komponen | shadcn/ui (Radix primitives) |
| Ikon | lucide-react |
| Notifikasi toast | sonner |
| Format uang/tanggal | Intl.NumberFormat / Intl.DateTimeFormat (`id-ID`, Asia/Jakarta) |
| Testing | Vitest + React Testing Library |
| Lint | ESLint 9 + Prettier |

### Struktur frontend

```
frontend/
├── src/
│   ├── app/              # router, providers, layout
│   ├── features/
│   │   ├── auth/
│   │   ├── catalog/
│   │   ├── cart/
│   │   ├── checkout/     # checkout, halaman sukses, lacak pesanan (M3)
│   │   ├── orders/       # riwayat pesanan akun (M5)
│   │   └── admin/
│   ├── components/ui/    # shadcn
│   ├── lib/              # api client, formatter, env
│   └── styles/
├── public/
├── index.html
├── vite.config.ts
└── package.json
```

### Konvensi frontend

- Mobile-first; breakpoint utama `sm`, `md`, `lg`.
- API client tunggal (`lib/api.ts`) dengan penanganan 401 → refresh sekali → ulangi.
- Tidak menyimpan token di localStorage bila memungkinkan; **refresh token di cookie HttpOnly**, access token di memori. (Guest cart tetap di localStorage.)
- Harga dari client hanya untuk tampilan; server menghitung ulang.
- Komponen admin dipisah folder `features/admin`, dilindungi `<RequireRole role="admin">`.

---

## 4. Infrastruktur lokal

| Container | Image | Port | Fungsi |
|---|---|---|---|
| `postgres` | `postgres:17-alpine` | 5433 (host) | 6 database (5 service + notification) |
| `kafka` | `apache/kafka:3.9.0` (KRaft) | 29092 (host) | broker |
| `kafka-ui` | `provectuslabs/kafka-ui:v0.7.2` | 8090 (host) | inspeksi topik |
| `mailpit` (opsional) | `axllent/mailpit` | 8025 | uji email |

Profil compose `observability`: `prometheus`, `grafana`.
Profil compose `prod`: `caddy`.

### Environment variables (contoh)

```dotenv
# umum
APP_ENV=development
LOG_LEVEL=debug
TZ=Asia/Jakarta

# gateway
GATEWAY_HTTP_ADDR=:8081
JWT_SECRET=change-me
JWT_ACCESS_TTL=15m
JWT_REFRESH_TTL=720h
GATEWAY_CORS_ORIGINS=http://localhost:5173

# auth
AUTH_GRPC_ADDR=:50051
AUTH_DB_DSN=postgres://saybu:saybu@localhost:5433/saybu_auth?sslmode=disable

# catalog
CATALOG_GRPC_ADDR=:50052
CATALOG_DB_DSN=postgres://saybu:saybu@localhost:5433/saybu_catalog?sslmode=disable

# shipping
SHIPPING_GRPC_ADDR=:50053
SHIPPING_DB_DSN=postgres://saybu:saybu@localhost:5433/saybu_shipping?sslmode=disable

# order
ORDER_GRPC_ADDR=:50054
ORDER_DB_DSN=postgres://saybu:saybu@localhost:5433/saybu_orders?sslmode=disable
ORDER_EXPIRY=24h
ORDER_EXPIRY_SWEEP=1m

# payment
PAYMENT_GRPC_ADDR=:50055
PAYMENT_DB_DSN=postgres://saybu:saybu@localhost:5433/saybu_payments?sslmode=disable
UPLOAD_DIR=./data/uploads
UPLOAD_MAX_BYTES=5242880

# notification
NOTIFICATION_HTTP_ADDR=:8086
NOTIFICATION_DB_DSN=postgres://saybu:saybu@localhost:5433/saybu_notifications?sslmode=disable
TELEGRAM_BOT_TOKEN=
TELEGRAM_ADMIN_CHAT_ID=

# kafka
KAFKA_BROKERS=localhost:29092
KAFKA_CONSUMER_GROUP_PREFIX=saybu

# notification
NOTIFICATION_HTTP_ADDR=:8086
TELEGRAM_BOT_TOKEN=
TELEGRAM_ADMIN_CHAT_ID=

# bank (instruksi transfer manual di halaman sukses; nilai di-quote bila ada '&')
BANK_NAME="BCA"
BANK_ACCOUNT="1234567890"
BANK_HOLDER="Saybu Buah & Sayur"
```

### Environment frontend (Vite)

Dibaca dari `frontend/.env` (lihat `frontend/.env.example`).

```dotenv
VITE_API_BASE_URL=/api/v1
VITE_PROXY_TARGET=http://localhost:8081

# Instruksi transfer — sumber utama dari API pembayaran (GET /orders/{code}/payment).
# Nilai ini hanya fallback bila API tidak mengirim data bank.
VITE_BANK_NAME="BCA"
VITE_BANK_ACCOUNT="1234567890"
VITE_BANK_HOLDER="Saybu Buah & Sayur"
```

---

## 5. Makefile (target wajib)

| Target | Fungsi |
|---|---|
| `make help` | Daftar target |
| `make up` | `docker compose up -d` (postgres, kafka, kafka-ui) |
| `make down` | Hentikan container |
| `make logs` | Ikuti log container |
| `make ps` | Status container |
| `make proto` | `buf lint` + `buf generate` |
| `make migrate` | Migrasi semua DB |
| `make seed` | Data contoh (kategori, produk, zona, admin) |
| `make dev` | Jalankan semua service Go (paralel) |
| `make dev-frontend` | `npm run dev` di `frontend/` |
| `make test` | Unit test |
| `make test-integration` | Test integrasi Kafka (butuh infra hidup) |
| `make lint` | `go vet` + golangci-lint (bila ada) |
| `make fmt` | `gofmt -w` |
| `make build` | Build semua binary ke `backend/bin` |
| `make observability` | Jalankan Prometheus + Grafana (profil `observability`) |
| `make observability-down` | Hentikan Prometheus + Grafana |
| `make load-test` | Load test k6 via Docker (100 VU katalog + 10 checkout/detik) |
| `make backup` | Backup semua DB ke `backups/<timestamp>` |
| `make restore DIR=...` | Restore semua DB dari hasil `make backup` |
| `make clean` | Hapus binary & volume container |

---

## 6. Port map

| Komponen | Port |
|---|---|
| Frontend dev | 5173 |
| Gateway REST | 8081 |
| auth gRPC / admin | 50051 / 9101 |
| catalog gRPC / admin | 50052 / 9102 |
| shipping gRPC / admin | 50053 / 9103 |
| order gRPC / admin | 50054 / 9104 |
| payment gRPC / admin | 50055 / 9105 |
| notification HTTP | 8086 |
| Postgres (host) | 5433 |
| Kafka host | 29092 |
| Kafka internal | 9092 |
| Kafka UI | 8090 |
| Prometheus | 9090 |
| Grafana | 3000 |
| Mailpit | 8025 |

---

## 7. Alur kerja lokal

```bash
cp .env.example .env
make up                 # infra
make test               # unit test
make test-integration   # test Kafka (infra harus hidup)
make proto              # generate gRPC (saat proto berubah)
make dev                # backend
make dev-frontend       # terminal lain
```

Buka `http://localhost:5173` (toko) dan `http://localhost:5173/admin` (panel).
Kafka UI: `http://localhost:8090`. Health service: `:8081`, `:9101`–`:9105`, `:8086`.
Observability (opsional): `make observability` → Prometheus `http://localhost:9090`, Grafana
`http://localhost:3000` (`admin`/`admin`). Alert rules + contact point Telegram ter-provision
di folder Grafana `Saybu Alerts` (butuh `TELEGRAM_BOT_TOKEN` & `TELEGRAM_ADMIN_CHAT_ID`).

---

## 8. CI/CD

**CI (setiap PR)** — sesuai `.github/workflows/ci.yml`
1. `go vet ./...`
2. `go test ./... -race -count=1`
3. `go build ./...`
4. `buf lint` + `buf generate` + cek `git diff --exit-code` (generated code harus sinkron)
5. `docker compose config -q`
6. Frontend (M1+): `npm ci`, `eslint`, `tsc --noEmit`, `vitest run`, `npm run build`

**CD (merge ke `main`, M7)**
1. Build & push image ke GHCR dengan tag commit SHA
2. SSH ke VPS → `docker compose pull` → `docker compose up -d --remove-orphans`
3. Jalankan migrasi job
4. Health check; rollback ke tag sebelumnya bila gagal

---

## 9. Standar kualitas

- Cakupan uji domain ≥ 70%.
- Tidak ada secret di repo; `.env.example` lengkap.
- Setiap endpoint baru: dokumentasi di `API_SPEC.md` + test.
- Setiap event baru: dokumentasi di `ARSITEKTUR.md` §5 + test consumer.
- Setiap milestone selesai: perbarui `TASK.md` dan catat keputusan baru di ADR.
