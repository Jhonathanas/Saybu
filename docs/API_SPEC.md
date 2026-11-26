# API SPEC — Saybu

| Field | Value |
|---|---|
| Versi | 1.0 |
| Base URL lokal | `http://localhost:8081/api/v1` |
| Base URL produksi | `https://<domain>/api/v1` |
| Format | JSON (`application/json`), kecuali upload `multipart/form-data` |
| Terakhir diperbarui | 2026-11-12 |

> Dokumen ini adalah **kontrak**. Setiap perubahan wajib disertai pembaruan di sini + test.

---

## 1. Konvensi

### 1.1 Autentikasi

- `Authorization: Bearer <access_token>` untuk endpoint terproteksi.
- Access token JWT, umur 15 menit.
- Refresh token disimpan di cookie `saybu_rt` (`HttpOnly`, `Secure`, `SameSite=Lax`, path `/api/v1/auth`).
- Guest dapat mengakses katalog, ongkir, checkout, dan pelacakan pesanan.

### 1.2 Header wajib/umum

| Header | Kapan | Keterangan |
|---|---|---|
| `Authorization` | endpoint terproteksi | Bearer token |
| `Idempotency-Key` | `POST /orders` | UUID v4 dari client |
| `X-Request-Id` | opsional | Bila kosong, gateway membuat sendiri |
| `Accept-Language` | opsional | Hanya `id` untuk saat ini |

### 1.3 Format respons sukses

```json
{
  "data": { },
  "meta": { }
}
```

`meta` hanya ada pada list/paginasi.

### 1.4 Format error

```json
{
  "error": {
    "code": "VALIDATION_ERROR",
    "message": "Data yang dikirim tidak valid.",
    "details": [{ "field": "customer.phone", "message": "Nomor HP tidak valid." }],
    "request_id": "01J..."
  }
}
```

| HTTP | `code` | Kapan |
|---|---|---|
| 400 | `VALIDATION_ERROR` | Input tidak lolos validasi |
| 401 | `UNAUTHORIZED` | Token tidak ada/kedaluwarsa |
| 403 | `FORBIDDEN` | Role tidak cukup |
| 404 | `NOT_FOUND` | Resource tidak ada |
| 409 | `CONFLICT` | Stok berubah, transisi status ilegal, duplikat |
| 422 | `BUSINESS_RULE` | Aturan bisnis dilanggar (mis. di luar zona) |
| 429 | `RATE_LIMITED` | Terlalu banyak permintaan |
| 500 | `INTERNAL` | Kesalahan tak terduga |
| 503 | `UNAVAILABLE` | Dependensi (DB/Kafka) tidak siap |

### 1.5 Paginasi

Query: `?page=1&limit=20` (`limit` maks 100).

```json
{ "data": [], "meta": { "page": 1, "limit": 20, "total": 42, "total_pages": 3 } }
```

### 1.6 Uang, jumlah, waktu

- Semua nominal uang: **integer rupiah** (`75000` = Rp75.000). Tidak ada desimal.
- `qty` produk: **integer dalam satuan terkecil** (gram untuk `kg`, pcs untuk lainnya). Contoh 0,5 kg → `qty: 500`.
- Waktu: ISO-8601 UTC (`2026-11-11T03:15:30Z`). UI menampilkan WIB.

### 1.7 Mapping gRPC → HTTP (di gateway)

| gRPC code | HTTP | `error.code` |
|---|---|---|
| `InvalidArgument` | 400 | `VALIDATION_ERROR` |
| `Unauthenticated` | 401 | `UNAUTHORIZED` |
| `PermissionDenied` | 403 | `FORBIDDEN` |
| `NotFound` | 404 | `NOT_FOUND` |
| `AlreadyExists` / `Aborted` | 409 | `CONFLICT` |
| `FailedPrecondition` | 422 | `BUSINESS_RULE` |
| `ResourceExhausted` | 429 | `RATE_LIMITED` |
| `Unavailable` / `DeadlineExceeded` | 503 | `UNAVAILABLE` |
| `Internal` | 500 | `INTERNAL` |

---

## 2. Auth

### 2.1 `POST /auth/register`

```json
{ "name": "Siti", "phone": "081234567890", "email": "siti@example.com", "password": "rahasia123" }
```

- `phone` wajib, format `^08[0-9]{8,11}$`; `email` opsional.
- `password` minimal 8 karakter.

**201**
```json
{
  "data": {
    "user": { "id": "0192...", "name": "Siti", "phone": "081234567890", "email": "siti@example.com", "role": "customer" },
    "access_token": "eyJ...", "expires_in": 900
  }
}
```
Set-Cookie: `saybu_rt=...; HttpOnly; Secure; SameSite=Lax; Path=/api/v1/auth; Max-Age=2592000`

### 2.2 `POST /auth/login`

```json
{ "identifier": "081234567890", "password": "rahasia123" }
```
`identifier` boleh email atau nomor HP. **200** sama seperti register. **401** bila salah.

### 2.3 `POST /auth/refresh`

Tanpa body, memakai cookie. **200** → access token baru + cookie refresh baru (rotasi).
Bila token sudah dipakai/dicabut → **401** dan semua sesi user dicabut.

### 2.4 `POST /auth/logout`

Cabut refresh token aktif, hapus cookie. **204**.

### 2.5 `GET /me` 🔒

**200**
```json
{ "data": { "id": "...", "name": "Siti", "phone": "...", "email": "...", "role": "customer" } }
```

---

## 3. Katalog (publik)

### 3.1 `GET /categories`

```json
{ "data": [ { "id": "...", "name": "Buah", "slug": "buah", "sort": 1 } ] }
```

### 3.2 `GET /products`

Query: `category` (slug), `q` (cari nama), `sort` = `newest|price_asc|price_desc`, `page`, `limit`.

Hanya produk aktif yang dikembalikan. Untuk satuan `kg`, `stock` adalah jumlah dalam **gram**
(satuan terkecil); `price` tetap harga per kg. Satuan lain (`ikat`, `pcs`, `buah`, `paket`)
menyimpan stok per satuan.

**200**
```json
{
  "data": [
    {
      "id": "0192...",
      "slug": "bayam-segar",
      "name": "Bayam Segar",
      "category": { "id": "...", "name": "Sayur", "slug": "sayur" },
      "description": "Bayam hijau segar panen pagi.",
      "unit": "ikat",
      "price": 5000,
      "stock": 40,
      "min_qty": 1,
      "step_qty": 1,
      "is_available": true,
      "images": ["/uploads/2026/11/0192....jpg"]
    }
  ],
  "meta": { "page": 1, "limit": 20, "total": 1, "total_pages": 1 }
}
```

### 3.3 `GET /products/{slug}`

Sama seperti item di atas + `created_at`, `updated_at`. **404** bila tidak ada/nonaktif.

---

## 4. Ongkir

### 4.1 `GET /shipping/zones`

```json
{
  "data": [
    { "id": "...", "name": "Kec. Sukamaju", "areas": ["Sukamaju", "Sukamulya"], "fee": 10000, "free_min_subtotal": 150000 }
  ]
}
```

### 4.2 `GET /shipping/quote?zone_id=...&subtotal=75000`

**200**
```json
{ "data": { "zone_id": "...", "zone_name": "Kec. Sukamaju", "fee": 10000, "free_shipping": false, "subtotal": 75000, "total": 85000 } }
```

**422** bila zona tidak dikenal.

---

## 5. Pesanan

### 5.1 `POST /orders` (guest & akun)

Header: `Idempotency-Key: <uuid>`

```json
{
  "customer": { "name": "Siti", "phone": "081234567890", "email": "siti@example.com", "note": "Tolong dipilih yang segar" },
  "fulfillment": "DELIVERY",
  "zone_id": "0192...",
  "address": { "line": "Jl. Melati 12", "rt_rw": "003/004", "kelurahan": "Sukamaju", "kecamatan": "Sukamaju", "city": "Bandung", "postal_code": "40123", "landmark": "Depan masjid" },
  "payment_method": "TRANSFER",
  "items": [
    { "product_id": "0192...", "qty": 2 },     // 2 ikat bayam
    { "product_id": "0193...", "qty": 1000 },  // 1 kg tomat
    { "product_id": "0194...", "qty": 2000 }   // 2 kg jeruk
  ]
}
```

Aturan:
- `fulfillment=PICKUP` → `zone_id` & `address` diabaikan, ongkir 0.
- `payment_method`: `TRANSFER` | `COD`.
- Server **menghitung ulang** harga, stok, dan ongkir. Harga dari client tidak pernah dipakai.
- `qty` harus kelipatan `step_qty` (default 1 untuk pcs/ikat, 100 untuk kg) dan ≥ `min_qty`.

**201**
```json
{
  "data": {
    "id": "0192...",
    "code": "SB-20261111-0007",
    "status": "PENDING_PAYMENT",
    "fulfillment": "DELIVERY",
    "payment_method": "TRANSFER",
    "subtotal": 75000,
    "shipping_fee": 10000,
    "total": 85000,
    "items": [
      { "product_id": "0192...", "name": "Bayam Segar", "unit": "ikat", "qty": 2, "qty_display": "2 ikat", "unit_price": 5000, "line_total": 10000 },
      { "product_id": "0193...", "name": "Tomat", "unit": "kg", "qty": 1000, "qty_display": "1 kg", "unit_price": 15000, "line_total": 15000 },
      { "product_id": "0194...", "name": "Jeruk", "unit": "kg", "qty": 2000, "qty_display": "2 kg", "unit_price": 25000, "line_total": 50000 }
    ],
    "expires_at": "2026-11-12T03:15:30Z",
    "created_at": "2026-11-11T03:15:30Z"
  }
}
```

**409** `CONFLICT` bila stok tidak cukup (sertakan `details` produk yang gagal).
**422** bila di luar zona atau `Idempotency-Key` dipakai dengan body berbeda.

### 5.2 `GET /orders/{code}?phone=081234567890`

Pelacakan guest. `phone` harus cocok dengan pesanan. Bila login sebagai pemilik, `phone` opsional.

**200**
```json
{
  "data": {
    "code": "SB-20261111-0007",
    "status": "WAITING_VERIFICATION",
    "status_history": [
      { "status": "PENDING_PAYMENT", "at": "2026-11-11T03:15:30Z" },
      { "status": "WAITING_VERIFICATION", "at": "2026-11-11T03:20:00Z" }
    ],
    "subtotal": 75000, "shipping_fee": 10000, "total": 85000,
    "payment_method": "TRANSFER",
    "items": [ ],
    "expires_at": "..."
  }
}
```

### 5.3 `GET /me/orders` 🔒

Daftar pesanan milik user, paginasi, urut terbaru.

### 5.4 `POST /orders/{code}/cancel`

Body: `{ "phone": "081234567890", "reason": "Berubah pikiran" }` (guest) atau kosong bila login.
Boleh selama status `PENDING_PAYMENT` / `WAITING_VERIFICATION` / `PAID`.
**200** → status `CANCELLED`. **409** bila sudah diproses.

---

## 6. Pembayaran

> **Status implementasi:** endpoint di §6 sudah ada (M4). Instruksi transfer & status pembayaran
> dibaca dari §6.1; frontend memakai `VITE_BANK_*` hanya sebagai fallback bila API tidak
> mengirim `bank`. Pembayaran dibuat otomatis dari event `order.created` (§9.1), jadi tidak ada
> endpoint "buat pembayaran".

Akses §6.1 & §6.2: pemilik akun memakai token (tanpa `phone`); guest wajib menyertakan `phone`
yang cocok dengan pesanan (404 bila tidak cocok).

### 6.1 `GET /orders/{code}/payment`

Instruksi pembayaran + status.

```json
{
  "data": {
    "method": "TRANSFER",
    "status": "UNPAID",
    "amount": 85000,
    "bank": { "name": "BCA", "account_number": "1234567890", "account_holder": "Nama Pemilik" },
    "proof_url": null,
    "verified_at": null,
    "note": null
  }
}
```

`status`: `UNPAID` | `WAITING_VERIFICATION` | `PAID` | `REJECTED` | `CANCELLED` | `COD_PENDING` | `COD_PAID`.
`bank` hanya terisi untuk metode `TRANSFER` (null untuk COD). `note` berisi alasan saat `REJECTED`.

### 6.2 `POST /orders/{code}/payment-proof`

`multipart/form-data`:
- `file` (wajib): JPEG/PNG/PDF, maks 5 MB (validasi ekstensi + tipe konten di gateway)
- `phone`: wajib untuk guest (diabaikan bila pemilik memakai token)

**200** → `{ "data": { "status": "WAITING_VERIFICATION", "proof_url": "/uploads/..." } }`
**409** bila status pembayaran bukan `UNPAID` atau `REJECTED` (mis. sudah menunggu verifikasi).
Rate limit 5/menit per IP.

### 6.3 Verifikasi (admin)

`GET /admin/payments?status=WAITING_VERIFICATION&page=1&limit=20`

```json
{
  "data": [
    {
      "id": "...",
      "order_code": "SB-...",
      "customer_name": "Siti",
      "customer_phone": "0812...",
      "method": "TRANSFER",
      "amount": 85000,
      "proof_url": "...",
      "uploaded_at": "...",
      "status": "WAITING_VERIFICATION"
    }
  ],
  "meta": { "page": 1, "limit": 20, "total": 3 }
}
```

`status` opsional (filter). `POST /admin/payments/{id}/verify`

```json
{ "decision": "APPROVE", "note": "Dana masuk 11/11 10.25" }
```

`decision`: `APPROVE` | `REJECT`. **200** → status pembayaran & pesanan diperbarui (lewat event
`payment.confirmed`/`payment.rejected`, §9.2). `409` bila status bukan `WAITING_VERIFICATION`.
Saat `REJECT`, `note` sebaiknya diisi sebagai alasan untuk pelanggan.

---

## 7. Admin — produk, pesanan, statistik

Semua endpoint `/admin/*` butuh role `admin` (401/403 bila tidak).

### 7.1 `POST /admin/products`

```json
{
  "name": "Bayam Segar", "slug": "bayam-segar", "category_id": "...",
  "description": "Bayam hijau segar panen pagi.",
  "unit": "ikat", "price": 5000, "stock_qty": 40,
  "min_qty": 1, "step_qty": 1, "is_active": true,
  "images": ["/uploads/2026/11/....jpg"]
}
```

**201** → objek produk admin (termasuk `stock_qty`, `is_active`).

Bentuk objek produk admin (dipakai §7.1–§7.4):

```json
{
  "id": "0192...", "slug": "bayam-segar", "name": "Bayam Segar",
  "category": { "id": "...", "name": "Sayur", "slug": "sayur" }, "category_id": "...",
  "description": "Bayam hijau segar panen pagi.",
  "unit": "ikat", "price": 5000, "stock_qty": 40,
  "min_qty": 1, "step_qty": 1, "is_active": true,
  "images": ["/uploads/2026/11/....jpg"],
  "created_at": "2026-11-13T07:50:17+07:00", "updated_at": "2026-11-13T07:50:17+07:00"
}
```

`slug` boleh dikosongkan saat create; server membuatnya dari `name` dan menjamin keunikan.
Untuk satuan `kg`, `price` per kg dan `stock_qty`/`qty` dalam gram.

### 7.2 `PATCH /admin/products/{id}`

Field parsial; hanya field yang dikirim yang diubah. `price` dan `stock_qty` boleh sekaligus.
Mengirim `images` akan **mengganti** seluruh daftar gambar.

### 7.3 `PATCH /admin/products/{id}/stock`

```json
{ "mode": "set", "qty": 25 }
```
`mode`: `set` | `adjust` (`qty` boleh negatif). Stok tidak boleh < 0 → **422**.

### 7.4 `DELETE /admin/products/{id}`

Soft delete (`is_active=false`). **204**.

### 7.5 `POST /admin/uploads`

`multipart/form-data` field `file`. Tipe JPG/PNG/WEBP dan ≤ 5 MB (default) divalidasi server.
**201** → `{ "data": { "url": "/uploads/2026/11/uuid.jpg" } }`. **400** bila tipe/ukuran tidak valid.

### 7.6 `GET /admin/products`

Daftar produk untuk panel admin, **termasuk produk nonaktif** (berbeda dari §3.2).
Query: `category` (slug), `q` (cari nama), `sort`, `page`, `limit`. Respons memakai bentuk
objek produk admin di §7.1.

### 7.7 `GET /admin/orders`

Query: `status`, `date_from`, `date_to`, `q`, `page`, `limit`. Respons berhalaman (`data` + `meta`).

`GET /admin/orders/{id}` mengembalikan satu pesanan (bentuk sama dengan §5.2) untuk panel admin.

### 7.8 `PATCH /admin/orders/{id}/status`

```json
{ "status": "PREPARING", "note": "Sedang disiapkan" }
```
Transisi divalidasi. **409** bila ilegal.

### 7.9 `GET /admin/stats/today`

```json
{
  "data": {
    "orders_count": 12,
    "revenue_paid": 845000,
    "awaiting_verification": 3,
    "low_stock": [ { "product_id": "...", "name": "Tomat", "stock": 3, "unit": "kg" } ]
  }
}
```

### 7.10 Zona ongkir (admin)

- `GET /admin/shipping/zones` — termasuk zona nonaktif
- `POST /admin/shipping/zones`
- `PATCH /admin/shipping/zones/{id}`
- `DELETE /admin/shipping/zones/{id}`

Body: `{ "name": "...", "areas": ["..."], "fee": 10000, "free_min_subtotal": 150000, "is_active": true }`

Respons (admin) menambahkan `is_active`, `created_at`, `updated_at`; `PATCH` dengan `is_active=false`
menonaktifkan zona tanpa menghapusnya (zona nonaktif tidak muncul di endpoint publik).

---

## 8. Kontrak gRPC internal

Lokasi proto: `backend/proto/saybu/<service>/v1/*.proto`.

### 8.1 `auth.v1.AuthService`

```proto
service AuthService {
  rpc Register(RegisterRequest) returns (AuthResponse);
  rpc Login(LoginRequest) returns (AuthResponse);
  rpc Refresh(RefreshRequest) returns (AuthResponse);
  rpc Logout(LogoutRequest) returns (google.protobuf.Empty);
  rpc GetUser(GetUserRequest) returns (User);
  rpc ListUsers(ListUsersRequest) returns (ListUsersResponse); // admin
  rpc RecordAudit(RecordAuditRequest) returns (google.protobuf.Empty); // internal, dari gateway
}
message User { string id=1; string name=2; string phone=3; string email=4; string role=5; }
message RecordAuditRequest {
  string actor_id=1; string actor_role=2; string action=3;
  string path=4; int32 status=5; string request_id=6;
}
```

`RecordAudit` hanya dipakai gateway (bukan endpoint REST). Field wajib: `actor_id`,
`actor_role`, `action`, `path`, dan `status` (100–599); `request_id` boleh kosong dan
dipakai sebagai kunci idempotensi saat retry. Audit ditulis ke tabel `audit_logs`
di database `saybu_auth`. Gateway memanggil dengan timeout 3 detik dan tetap
mengembalikan respons jika penyimpanan gagal (kesalahan hanya dicatat di log).

### 8.2 `catalog.v1.CatalogService`

```proto
service CatalogService {
  rpc ListCategories(ListCategoriesRequest) returns (ListCategoriesResponse);
  rpc ListProducts(ListProductsRequest) returns (ListProductsResponse);
  rpc GetProduct(GetProductRequest) returns (Product);
  rpc CreateProduct(CreateProductRequest) returns (Product);
  rpc UpdateProduct(UpdateProductRequest) returns (Product);
  rpc SetStock(SetStockRequest) returns (Product);
  rpc DeleteProduct(DeleteProductRequest) returns (google.protobuf.Empty);
  rpc CheckStock(CheckStockRequest) returns (CheckStockResponse);
  rpc ReserveStock(ReserveStockRequest) returns (ReserveStockResponse);
  rpc ReleaseStock(ReleaseStockRequest) returns (google.protobuf.Empty);
}
message StockItem { string product_id=1; int64 qty=2; }
```

### 8.3 `shipping.v1.ShippingService`

```proto
service ShippingService {
  rpc ListZones(ListZonesRequest) returns (ListZonesResponse);
  rpc Quote(QuoteRequest) returns (QuoteResponse);
  rpc UpsertZone(UpsertZoneRequest) returns (Zone);
  rpc DeleteZone(DeleteZoneRequest) returns (google.protobuf.Empty);
}
message QuoteRequest { string zone_id=1; int64 subtotal=2; }
message QuoteResponse { int64 fee=1; bool free_shipping=2; string zone_name=3; int64 total=4; }
```

### 8.4 `order.v1.OrderService`

```proto
service OrderService {
  rpc CreateOrder(CreateOrderRequest) returns (Order);
  rpc GetOrderByCode(GetOrderByCodeRequest) returns (Order);
  rpc ListMyOrders(ListMyOrdersRequest) returns (ListOrdersResponse);
  rpc GetOrderByID(GetOrderByIDRequest) returns (Order);           // admin (M5)
  rpc ListOrders(ListOrdersRequest) returns (ListOrdersResponse); // admin (M5)
  rpc UpdateStatus(UpdateStatusRequest) returns (Order);           // admin (M5)
  rpc CancelOrder(CancelOrderRequest) returns (Order);
  rpc GetTodayStats(GetTodayStatsRequest) returns (TodayStats);   // admin (M5)
}
```

> **M3:** proto hanya mengimplementasikan `CreateOrder`, `GetOrderByCode`, `ListMyOrders`, dan `CancelOrder`.
> RPC admin `GetOrderByID`/`ListOrders`/`UpdateStatus`/`GetTodayStats` ditambahkan pada M5. Job penandaan
> pesanan kedaluwarsa berjalan lewat ticker internal service order (tanpa RPC).

### 8.5 `payment.v1.PaymentService`

```proto
service PaymentService {
  rpc GetPaymentByOrderCode(GetPaymentByOrderCodeRequest) returns (Payment);
  rpc UploadProof(UploadProofRequest) returns (Payment);
  rpc ListPayments(ListPaymentsRequest) returns (ListPaymentsResponse);
  rpc VerifyPayment(VerifyPaymentRequest) returns (Payment);
}
```

> **M4 (penyimpangan dari rencana chunked):** `UploadProof` **unary**, bukan `stream`. Gateway yang
> menyimpan berkas bukti ke disk (validasi ekstensi + `http.DetectContentType`, batas 5 MB) lalu memanggil
> `UploadProof(code, phone, user_id, proof_url, size_bytes, content_type)`. Service pembayaran hanya
> menyimpan URL, tidak menangani byte berkas. Alasan: satu jalur unggah & rate limit di gateway, selaras
> dengan unggah gambar katalog (§7.5). `ListPendingPayments` dinamai `ListPayments` (mendukung filter status).

---

## 9. Event Kafka

Envelope (semua event):

```json
{
  "event_id": "0192...", "type": "order.created", "version": 1,
  "occurred_at": "2026-11-11T03:15:30Z",
  "aggregate_id": "0192...", "correlation_id": "req-...", "producer": "order-svc",
  "payload": {}
}
```

### 9.1 `order.events.v1`

```jsonc
// order.created
{
  "order_id": "...", "code": "SB-20261111-0007",
  "user_id": "0192...",                       // kosong untuk guest (M4)
  "customer": { "name": "Siti", "phone": "0812..." },
  "fulfillment": "DELIVERY", "payment_method": "TRANSFER",
  "zone_name": "Kec. Sukamaju", "address_line": "Jl. Melati 12",
  "subtotal": 75000, "shipping_fee": 10000, "total": 85000,
  "items": [{ "product_id": "...", "name": "Bayam Segar", "unit": "ikat", "qty": 2, "unit_price": 5000 }],
  "expires_at": "2026-11-12T03:15:30Z"
}

// order.status_changed
{ "order_id": "...", "code": "SB-...", "from": "PAID", "to": "PREPARING", "by": "admin-uuid", "note": "..." }

// order.cancelled
{ "order_id": "...", "code": "SB-...", "reason": "Berubah pikiran", "by": "customer", "items": [] }

// order.expired
{ "order_id": "...", "code": "SB-...", "items": [] }
```

> **M3** memancarkan `order.created` (saat checkout) serta `order.cancelled` dan `order.expired` (dari gateway
> dan job expire). Konsumer catalog (`catalog-svc`) bereaksi pada `order.cancelled`/`order.expired` untuk
> melepas reservasi stok secara idempoten.
>
> **M4** menambahkan `user_id` (aditif) pada `order.created` dan mulai memancarkan `order.status_changed`
> ketika status berubah karena event pembayaran (§9.2). `order.cancelled` dari pembatalan manual juga
> dikonsumsi service payment untuk menandai pembayaran `CANCELLED`.

### 9.2 `payment.events.v1`

```jsonc
// payment.proof_uploaded
{ "payment_id": "...", "order_id": "...", "code": "SB-...", "amount": 85000, "proof_url": "/uploads/..." }

// payment.confirmed
{ "payment_id": "...", "order_id": "...", "code": "SB-...", "amount": 85000, "verified_by": "admin-uuid" }

// payment.rejected
{ "payment_id": "...", "order_id": "...", "code": "SB-...", "reason": "Bukti tidak jelas" }
```

> **M4** memancarkan ketiga event dari service payment (lewat outbox). Konsumer: service order
> (`payment.proof_uploaded`/`confirmed`/`rejected` → transisi status pesanan + `order.status_changed`),
> service catalog (`payment.confirmed` → reservasi stok `CONFIRMED`), dan notification
> (semua event → Telegram). Semua konsumer idempoten via `processed_events`; notification menyimpan
> klaim di `saybu_notifications` (`pending` → `sent`, atau dihapus bila pengiriman gagal).

### 9.3 `catalog.events.v1`

Semua event ditulis lewat outbox pada transaksi yang sama (AGENTS.md §3.4).

```jsonc
// product.updated — create/update/soft-delete produk
{ "product_id": "...", "name": "Tomat", "slug": "tomat-merah", "price": 12000, "unit": "kg", "is_active": true }

// stock.low — dipancarkan saat stok melewati (turun ke) low_stock_threshold.
// Ditulis lewat outbox pada transaksi yang sama dengan pengurangan stok (M5),
// sehingga terkirim sekali per pelanggaran ambang. remaining dalam satuan terkecil.
{ "product_id": "...", "name": "Tomat", "remaining": 3, "unit": "kg", "threshold": 5 }

// stock.released — direncanakan M3 (saat pelepasan reservasi pesanan)
{ "order_id": "...", "items": [{ "product_id": "...", "qty": 500 }] }
```

Untuk satuan `kg`, `remaining`/`qty` dalam gram. `stock.released` belum dipancarkan pada M2.

---

## 10. Rate limit

| Endpoint | Batas |
|---|---|
| `POST /auth/login` | 5 / menit per IP+identifier |
| `POST /auth/register` | 3 / menit per IP |
| `POST /orders` | 10 / menit per IP |
| `POST /orders/*/payment-proof` | 5 / menit per IP |
| Endpoint tulis admin | 60 / menit per user |
| Endpoint baca | 120 / menit per IP |

Respons 429 menyertakan `Retry-After`.

---

## 11. Operasional (non-API publik)

| Endpoint | Port | Keterangan |
|---|---|---|
| `GET /healthz` | gateway `8081`, admin `9101`–`9105`, notification `8086` | Liveness |
| `GET /readyz` | sama | Readiness (DB/Kafka/downstream) |
| `GET /metrics` | sama | Prometheus |
| `GET /api/v1/version` | gateway `8081` | Nama service, versi, env |

Endpoint ini tidak untuk browser dan tidak memakai envelope `data`/`meta`.
