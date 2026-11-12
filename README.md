# Saybu

Toko online buah & sayur untuk usaha keluarga — sekaligus proyek latihan
**microservices Go + Kafka**.

- Frontend: React + TypeScript (Vite), mobile-first, panel admin di `/admin`
- Backend: 7 service Go (gateway, auth, catalog, shipping, order, payment, notification)
- Komunikasi: REST di gateway, gRPC antar service, event lewat Kafka
- Data: PostgreSQL (satu database per service)

## Dokumentasi

| Dokumen | Isi |
|---|---|
| [docs/PRD.md](docs/PRD.md) | Kebutuhan produk & alur bisnis |
| [docs/ARSITEKTUR.md](docs/ARSITEKTUR.md) | Diagram, Kafka, outbox, saga, ADR |
| [docs/TECH_STACK.md](docs/TECH_STACK.md) | Stack, konvensi, env, port |
| [docs/API_SPEC.md](docs/API_SPEC.md) | Kontrak REST, gRPC, event |
| [docs/TASK.md](docs/TASK.md) | Backlog per milestone |
| [AGENTS.md](AGENTS.md) | Aturan kerja untuk kontributor/AI agent |

## Status

| Milestone | Status |
|---|---|
| M0 Fondasi | ✅ selesai |
| M1 Auth & gateway | ⬜ berikutnya |
| M2 Katalog | ⬜ |
| M3 Order + Kafka | ⬜ |
| M4 Pembayaran & Telegram | ⬜ |
| M5–M7 | ⬜ |

## Prasyarat

- **Go 1.26 (Linux)** — cek dengan `go version`; harus `linux/amd64` atau `linux/arm64`.
  Go Windows (`go.exe`) di WSL tidak bisa dipakai build Linux.
- **Docker** + Compose v2
- **GNU Make**
- **buf** dan `protoc-gen-go`/`protoc-gen-go-grpc` (hanya bila mengubah `.proto`)
- Node 20+ (dibutuhkan mulai M1 untuk frontend)

## Mulai cepat

```bash
cp .env.example .env
make up          # Postgres, Kafka, Kafka UI
make test        # unit test
make test-integration  # test Kafka (infra harus hidup)
make dev         # jalankan semua service Go
```

Cek kesehatan:

```bash
curl localhost:8081/healthz   # gateway
curl localhost:9101/readyz    # auth (postgres)
curl localhost:8086/readyz    # notification (kafka)
```

Buka:

- Gateway: <http://localhost:8081/api/v1/version>
- Kafka UI: <http://localhost:8090>
- Frontend (M1+): <http://localhost:5173>

## Port

| Komponen | Port |
|---|---|
| Gateway REST | 8081 |
| auth gRPC / admin | 50051 / 9101 |
| catalog gRPC / admin | 50052 / 9102 |
| shipping gRPC / admin | 50053 / 9103 |
| order gRPC / admin | 50054 / 9104 |
| payment gRPC / admin | 50055 / 9105 |
| notification HTTP | 8086 |
| Postgres (host) | 5433 |
| Kafka (host) | 29092 |
| Kafka UI | 8090 |

> Postgres memakai **5433** di host karena 5432 sering dipakai proyek lain. Ubah lewat `POSTGRES_PORT` di `.env`.

## Struktur

```
backend/
├── go.mod                  # satu module: saybu
├── pkg/                    # library bersama
├── gen/                    # hasil buf generate (di-commit)
├── proto/saybu/<svc>/v1/
└── services/<svc>/cmd/main.go
frontend/                   # React SPA (M1+)
infra/                      # postgres init, script kafka, dev runner
docs/                       # dokumen proyek
```

## Perintah

```bash
make help
make up / down / logs / ps
make proto
make migrate / seed
make dev / dev-frontend
make test / test-integration
make lint / fmt / build
make clean        # hapus binary + volume (data hilang!)
```

## Troubleshooting

**Port bentrok (`port is already allocated`)**
Ubah di `.env`: `POSTGRES_PORT=5433`, `KAFKA_UI_PORT=8090`, atau hentikan container lain.

**Kafka tidak sehat**
`make logs` lalu tunggu ±30 detik. Cek `docker compose ps`.

**Service gagal start: environment variable wajib**
Pastikan `.env` ada (hasil `cp .env.example .env`) dan dijalankan dari root repo.

**`go: command not found` / versi Windows**
Pasang Go Linux dan pastikan `which go` menunjuk ke binary Linux, bukan `go.exe`.

**Reset total**
`make clean && make up` (menghapus semua data lokal).
