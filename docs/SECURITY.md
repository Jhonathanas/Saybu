# SECURITY — Saybu

| Field | Value |
|---|---|
| Versi | 1.0 |
| Terakhir diperbarui | 2026-11-18 |
| Cakupan | M6-05 (security review & hardening) |

Dokumen ini adalah **checklist keamanan** untuk Saybu. Setiap butir mencantumkan
status, lokasi implementasi, dan tindak lanjut bila ada. Perbarui saat kontrak
atau konfigurasi berubah.

Legenda status: **Aman** · **Sebagian** · **Tunda (M7)**.

---

## 1. Rahasia & konfigurasi

| Butir | Status | Catatan |
|---|---|---|
| Tidak ada secret di repo | Aman | `.env` di `.gitignore`; hanya `.env.example` yang di-commit tanpa nilai produksi |
| `JWT_SECRET` dari env, minimal 32 karakter | Aman | `authn.NewManager` menolak secret pendek |
| Password tidak pernah di-log | Aman | Argon2id di `pkg/authn`; log tidak memuat body request sensitif |
| Refresh token disimpan sebagai hash | Aman | Kolom `refresh_tokens.token_hash` |
| Bukti transfer tidak di-log | Aman | Handler hanya mencatat metadata (ukuran, tipe), bukan isi berkas |
| Rotasi secret produksi | Tunda (M7) | Ganti `JWT_SECRET`, `POSTGRES_PASSWORD`, kredensial Telegram saat deploy |

---

## 2. Autentikasi & otorisasi

| Butir | Status | Catatan |
|---|---|---|
| JWT akses umur pendek (15 menit) | Aman | `JWT_ACCESS_TTL` |
| Refresh rotasi + deteksi reuse | Aman | Reuse → cabut semua sesi user |
| Refresh token di cookie `HttpOnly`+`Secure`+`SameSite=Lax` | Aman | `httpx.SetRefreshCookie`; `Secure` aktif bila `APP_ENV=production` |
| Endpoint admin dilindungi role | Aman | `authmw.RequireRole("admin")` pada grup `/admin` |
| Service internal tidak diekspos publik | Aman | gRPC hanya bind lokal; hanya gateway yang menghadap browser |
| Otorisasi per-resource (pembeli hanya melihat pesanannya) | Aman | Cek `user_id`/`phone` di service order & payment |

---

## 3. Rate limiting

| Endpoint | Batas | Status |
|---|---|---|
| `POST /auth/login` | 5 / menit per IP+identifier | Aman |
| `POST /auth/register` | 3 / menit per IP | Aman |
| `POST /orders` | 10 / menit per IP | Aman |
| `POST /orders/*/payment-proof` | 5 / menit per IP | Aman |
| Endpoint tulis admin | 60 / menit per user | Aman |
| Endpoint baca | 120 / menit per IP | Aman |

- Implementasi: `services/gateway/internal/ratelimit` (fixed-window in-memory).
- Setiap 429 menyertakan header `Retry-After`.
- Bucket kedaluwarsa dibersihkan berkala (`nextSweep`) agar memori tidak tumbuh
  tanpa batas saat banyak IP.
- **Batasan:** in-memory per instance. Bila gateway di-scale > 1 replika, ganti
  dengan limiter berbasis Redis (catat di M7).

---

## 4. Header keamanan & CORS

| Header | Nilai | Status |
|---|---|---|
| `X-Content-Type-Options` | `nosniff` | Aman |
| `X-Frame-Options` | `DENY` | Aman |
| `Referrer-Policy` | `strict-origin-when-cross-origin` | Aman |
| `Permissions-Policy` | `geolocation=(), microphone=(), camera=()` | Aman |
| `Strict-Transport-Security` | `max-age=31536000; includeSubDomains` | Aktif hanya `APP_ENV=production` |
| `Content-Security-Policy` | `default-src 'self'; script-src 'self'; …` | Aktif di Caddy produksi (M7) |

- Implementasi: `httpx.SecurityHeaders(hsts)` dipasang di rantai middleware gateway.
- CORS: hanya origin pada `GATEWAY_CORS_ORIGINS` (default `http://localhost:5173`).
  `Access-Control-Allow-Headers` memuat `Authorization, Content-Type,
  Idempotency-Key, X-Request-Id, Accept-Language`.
- **CSP produksi:** dipasang di `infra/Caddyfile` (bukan gateway) karena SPA dilayani
  langsung oleh Caddy. Nilai `style-src 'self' 'unsafe-inline'` diperlukan untuk style
  dinamis React/Tailwind; `script-src 'self'` cukup karena build Vite tidak memakai inline script.

---

## 5. Validasi input & injeksi

| Butir | Status | Catatan |
|---|---|---|
| Harga/ongkir/total dihitung ulang di server | Aman | Client tidak dipercaya (aturan AGENTS.md #1) |
| Semua query pakai parameter (`$1`, `$2`, …) | Aman | `ListOrders` merakit klausa `WHERE` dari placeholder bernomor, bukan nilai; nilai selalu dikirim sebagai argumen |
| Tidak ada `fmt.Sprintf` dengan input pengguna pada SQL | Aman | `Sprintf` hanya menyusun potongan klausa/indeks placeholder |
| Validasi bentuk di gateway + bisnis di service | Aman | Domain mengembalikan `ValidationError` → 422 |
| Batas ukuran body | Aman | `http.MaxBytesReader` (login 1 MB; upload sesuai batas) |
| Dekode JSON ketat (satu objek) | Aman | `DisallowUnknownFields` + cek `io.EOF` |

---

## 6. Unggahan berkas

| Butir | Status | Catatan |
|---|---|---|
| Whitelist ekstensi | Aman | Gambar `.jpg/.jpeg/.png/.webp`; bukti + `.pdf` |
| Verifikasi tipe isi (`http.DetectContentType`) | Aman | Cek magic bytes, bukan hanya ekstensi |
| Batas ukuran | Aman | `maxUploadBytes`; bukti 5 MB |
| Nama file di-generate (UUID) | Aman | Tidak memakai nama dari client |
| Perlindungan path traversal | Aman | `storage.Disk.Path` menolak path di luar `UPLOAD_DIR` (`ErrInvalidPath`) |
| Berkas tidak dieksekusi | Aman | Disajikan sebagai statis (Caddy M7), bukan dijalankan |
| **TODO:** scan malware | Tunda (M7) | Opsional; volume rendah |

---

## 7. Logging & privasi

| Butir | Status | Catatan |
|---|---|---|
| Log JSON terstruktur dengan `request_id` | Aman | `slog` + `telemetry.RequestID` |
| Password/token/bukti tidak di-log | Aman | Diverifikasi di review |
| `request_id` dilacak lintas HTTP → gRPC → Kafka | Aman | `correlation_id` event diisi dari request id (M6-03) |
| Audit mutasi admin | Aman | `auditmw` mencatat `actor`, `action`, `path`, `status`, `request_id` |
| Retensi log | Tunda (M7) | Belum ada rotasi/retensi terpusat |

---

## 8. Database & jaringan

| Butir | Status | Catatan |
|---|---|---|
| Satu database per service | Aman | 5 DB terpisah; tidak ada join lintas service |
| Postgres tidak terekspos ke internet | Tunda (M7) | Saat ini bind host untuk dev; produksi hanya jaringan internal |
| Backup & uji restore | Aman | `make backup` / `make restore` (diuji M6-07, data utuh) |
| Least privilege DB | Tunda (M7) | Satu user `saybu` untuk semua DB di dev; produksi pisahkan user per service |

---

## 9. Dependensi

| Butir | Status | Catatan |
|---|---|---|
| Dependensi minimal & terpelihara | Aman | Lihat `TECH_STACK.md` §2.1 |
| `go vet` bersih | Aman | Bagian dari `make lint` |
| Pemindaian kerentanan otomatis | Aman | `govulncheck` + `npm audit --audit-level=high` di CI; Dependabot mingguan |

---

## 10. Ringkasan tindak lanjut (M7)

1. ~~Pasang CSP (via Caddy) dan HSTS~~ — selesai di `infra/Caddyfile` (produksi).
2. Ganti limiter in-memory dengan Redis bila gateway di-scale.
3. ~~Tutup port Postgres dari publik~~ — `docker-compose.prod.yml` tidak mem-publish 5432.
   Pisahkan user DB per service (least privilege) masih opsional.
4. ~~Tambah `govulncheck` + Dependabot di CI~~ — selesai.
5. Rotasi seluruh secret produksi; jangan pakai nilai contoh.
6. Opsional: pemindaian malware pada unggahan.

---

## Referensi

- `docs/ARSITEKTUR.md` §10 (Keamanan)
- `docs/API_SPEC.md` §10 (Rate limit)
- `backend/pkg/httpx/httpx.go` (`SecurityHeaders`, `CORS`, `ClientIP`)
- `backend/pkg/storage/storage.go` (validasi unggahan & path)
- `backend/services/gateway/internal/ratelimit/` (limiter + middleware)
