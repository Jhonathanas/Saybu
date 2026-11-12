# PRD — Saybu (Sayur & Buah)

| Field | Value |
|---|---|
| Versi | 1.0 (draft) |
| Status | Disetujui untuk M0 |
| Pemilik produk | Pemilik usaha (orangtua) + pengembang (anak) |
| Terakhir diperbarui | 2026-11-11 |

---

## 1. Ringkasan

**Saybu** adalah toko online untuk usaha buah dan sayur keluarga. Pembeli dapat melihat katalog,
memasukkan barang ke keranjang, memilih pengiriman (diantar atau ambil sendiri), dan menyelesaikan
pesanan secara **guest** (tanpa akun) maupun dengan akun. Pembayaran dilakukan **COD** atau
**transfer manual** yang diverifikasi admin. Setiap pesanan baru memicu notifikasi **Telegram** ke
pengelola.

Proyek ini sekaligus menjadi sarana latihan **backend microservices Go + Kafka** dengan standar
produksi (outbox, idempotensi, DLQ, observability).

### Masalah yang diselesaikan

1. Pesanan masuk berserakan di chat WhatsApp → sulit dilacak, rawan salah hitung.
2. Harga buah/sayur berubah harian → daftar harga manual cepat basi.
3. Tidak ada rekap pesanan, stok, dan pembayaran dalam satu tempat.
4. Pembeli tidak tahu stok tersedia atau ongkir ke wilayahnya.

### Tujuan (Goals)

- Satu alur pesanan digital dari katalog sampai selesai, tercatat rapi.
- Admin bisa mengubah produk, harga, dan stok sendiri dalam < 1 menit per produk.
- Notifikasi pesanan baru masuk ke Telegram dalam < 5 detik.
- Ongkir jelas per zona sebelum checkout.
- Menjadi contoh implementasi microservices Go yang layak portofolio.

### Non-goals (fase ini)

- Payment gateway otomatis (QRIS/Midtrans) — hanya transfer manual + COD.
- Aplikasi mobile native.
- Multi-toko / multi-vendor.
- Program loyalitas, kupon kompleks, langganan.
- Integrasi kurir pihak ketiga (hanya ongkir internal per zona).

---

## 2. Persona

| Persona | Deskripsi | Kebutuhan utama |
|---|---|---|
| **Pembeli guest** | Ibu rumah tangga / pekerja yang belanja dari HP | Cepat, tanpa daftar, harga & stok jelas, ongkir transparan |
| **Pembeli akun** | Pelanggan langganan | Riwayat pesanan, alamat tersimpan |
| **Admin / pengelola** | Orangtua (dan pemilik sebagai backup) | Update harga & stok cepat, lihat pesanan masuk, verifikasi transfer |
| **Sistem** | Job & consumer | Expire order, release stok, kirim notifikasi, retry |

---

## 3. User stories (prioritas MoSCoW)

### Must have

| ID | Sebagai | Saya ingin | Agar |
|---|---|---|---|
| US-01 | Pembeli | melihat daftar produk per kategori | menemukan barang cepat |
| US-02 | Pembeli | mencari produk berdasarkan nama | tidak scroll panjang |
| US-03 | Pembeli | melihat detail produk (foto, harga, satuan, stok) | yakin sebelum beli |
| US-04 | Pembeli | menambah produk ke keranjang dengan jumlah | belanja beberapa item |
| US-05 | Pembeli | checkout tanpa daftar akun | tidak ribet |
| US-06 | Pembeli | memilih zona ongkir dan melihat biayanya | tahu total sebelum bayar |
| US-07 | Pembeli | memilih COD atau transfer manual | sesuai kemampuan |
| US-08 | Pembeli | mengunggah bukti transfer | pesanan bisa diverifikasi |
| US-09 | Pembeli | melacak status pesanan dengan kode + no. HP | tahu progres tanpa akun |
| US-10 | Admin | login aman ke panel | mengelola toko |
| US-11 | Admin | CRUD produk, harga, stok, foto | menjaga katalog akurat |
| US-12 | Admin | melihat pesanan baru & detailnya | menyiapkan barang |
| US-13 | Admin | memverifikasi / menolak bukti transfer | memastikan uang masuk |
| US-14 | Admin | mengubah status pesanan | mengelola proses |
| US-15 | Admin | menerima notifikasi Telegram pesanan baru | tidak melewatkan order |

### Should have

| ID | Sebagai | Saya ingin | Agar |
|---|---|---|---|
| US-16 | Pembeli | mendaftar akun & melihat riwayat pesanan | belanja ulang cepat |
| US-17 | Pembeli | membatalkan pesanan sebelum diproses | tidak jadi beli |
| US-18 | Admin | melihat daftar pesanan dengan filter status | prioritas kerja jelas |
| US-19 | Admin | mengatur zona & tarif ongkir | ongkir akurat |
| US-20 | Admin | melihat ringkasan penjualan harian | evaluasi usaha |

### Could have

- Kupon diskon sederhana, produk unggulan, notifikasi stok menipis, pencarian fuzzy, rekomendasi produk.

### Won't have (sekarang)

- Payment gateway, kurir eksternal, mobile app, multi-bahasa.

---

## 4. Alur utama

```
Katalog → Keranjang → Checkout (data + zona ongkir) → Buat Pesanan (PENDING_PAYMENT)
   ├─ COD / ambil sendiri ─────────────► diproses admin
   └─ Transfer manual → upload bukti ──► WAITING_VERIFICATION → admin verifikasi ──► PAID
                                                                     └─ tolak ──► PENDING_PAYMENT (bisa upload ulang)
PAID → PREPARING → SHIPPED / READY_PICKUP → COMPLETED
                                        └─ gagal ──► CANCELLED
PENDING_PAYMENT lewat 24 jam ──► EXPIRED (stok dilepas)
```

---

## 5. Kebutuhan fungsional per modul

### 5.1 Katalog
- Kategori: minimal **Buah** dan **Sayur**; admin bisa menambah.
- Produk: nama, slug, deskripsi, kategori, foto (1–5), satuan (`kg`, `pcs`, `ikat`, `pack`),
  harga per satuan, stok, minimum pembelian, status aktif.
- Stok disimpan dalam **satuan terkecil** (gram untuk kg, pcs untuk lainnya) agar bebas desimal.
- Produk dengan stok 0 tampil "Habis" dan tidak bisa ditambahkan ke keranjang.
- Pencarian berdasarkan nama; filter kategori; urutkan berdasarkan terbaru/harga.

### 5.2 Keranjang
- Disimpan di browser (localStorage) untuk guest, disinkronkan ke akun saat login (opsional fase 2).
- Menampilkan subtotal, peringatan bila stok berubah atau produk nonaktif.
- Harga di keranjang **selalu divalidasi ulang di server** saat checkout.

### 5.3 Checkout
- Data pembeli: nama, no. HP (format Indonesia), email opsional, catatan.
- Metode pengiriman: **Diantar** (wajib zona) atau **Ambil sendiri** (ongkir 0).
- Metode bayar: **COD** atau **Transfer manual**.
- Alamat: jalan, RT/RW, kelurahan, kecamatan, kota, kode pos, patokan.
- Idempotensi: klik ganda tidak membuat dua pesanan (`Idempotency-Key`).
- Pesanan menyimpan **snapshot** nama, harga, satuan, dan gambar produk.

### 5.4 Ongkir
- Zona: nama, daftar kecamatan/kelurahan, tarif flat, minimum belanja gratis ongkir (opsional).
- Alamat di luar zona → checkout ditolak dengan pesan jelas.
- Fase lanjut: ongkir berbasis radius (Haversine dari titik toko).

### 5.5 Pembayaran
- Transfer manual: pembeli melihat instruksi rekening, mengunggah bukti (JPG/PNG/PDF, maks 5 MB).
- Admin menyetujui/menolak dengan catatan; penolakan mengembalikan status agar bisa upload ulang.
- COD: dibayar saat barang diterima; admin menandai lunas.
- Semua perubahan pembayaran menghasilkan event Kafka.

### 5.6 Pesanan
- Kode pesanan publik yang mudah dibaca, mis. `SB-20261111-0007`.
- Status: `PENDING_PAYMENT`, `WAITING_VERIFICATION`, `PAID`, `PREPARING`, `SHIPPED`,
  `READY_PICKUP`, `COMPLETED`, `CANCELLED`, `EXPIRED`.
- Transisi status divalidasi oleh state machine; transisi ilegal ditolak.
- Pembeli dapat membatalkan selama status belum `PREPARING`.
- Setiap perubahan status tercatat di `order_status_history`.

### 5.7 Notifikasi
- Telegram ke admin untuk: pesanan baru, bukti transfer diunggah, pembayaran terverifikasi,
  pesanan dibatalkan, stok menipis.
- Pesan berisi kode, total, metode bayar, ringkasan item, tautan ke panel admin.
- Gagal kirim → retry dengan backoff, lalu DLQ; tidak boleh menghambat alur pesanan.

### 5.8 Admin
- Login dengan email + password; role `admin`.
- Dashboard: pesanan hari ini, perlu verifikasi, stok menipis, penjualan hari ini.
- CRUD produk & kategori, upload foto, ubah stok cepat.
- Daftar & detail pesanan, filter status, ubah status, verifikasi pembayaran.
- Kelola zona ongkir.
- Audit ringan: siapa mengubah apa dan kapan (fase M5+).

---

## 6. Kebutuhan non-fungsional

| Aspek | Target |
|---|---|
| Performa | Halaman katalog < 2 s pada 4G; API p95 < 300 ms (lokal) |
| Ketersediaan | 99% cukup; downtime terjadwal untuk deploy diperbolehkan |
| Skala | 100–300 pesanan/bulan, 100 SKU, 5 admin; lonjakan 10x saat musim libur |
| Keamanan | Password argon2id, JWT pendek + refresh rotasi, rate limit login, validasi server, upload dibatasi tipe & ukuran |
| Privasi | Hanya data yang diperlukan; no. HP pembeli tidak ditampilkan publik |
| Aksesibilitas | Kontras cukup, target sentuh ≥ 44 px, form berlabel, bisa dipakai satu tangan |
| Mobile-first | Layout dioptimalkan untuk 360–430 px |
| Observability | Structured log JSON + request-id, metrik Prometheus, trace opsional |
| Backup | Dump Postgres harian, retensi 7 hari, uji restore bulanan |

---

## 7. Metrik keberhasilan

- ≥ 80% pesanan masuk lewat aplikasi (bukan chat manual) dalam 2 bulan.
- Waktu admin memproses pesanan baru < 5 menit.
- Nol pesanan ganda akibat double submit.
- Nol kejadian stok terjual melebihi persediaan.
- Semua pesanan punya jejak status yang lengkap.

---

## 8. Batasan & asumsi

- Satu lokasi toko, satu mata uang (IDR), satu bahasa (Indonesia), zona waktu Asia/Jakarta.
- Admin mengelola produk sendiri; tidak ada tim konten.
- Foto produk diunggah manual; kualitas secukupnya.
- Hosting akhir di satu VPS (≥ 4 GB RAM karena Kafka), development lokal di WSL2 + Docker.
- Pengiriman dilakukan sendiri oleh keluarga, bukan kurir pihak ketiga.

---

## 9. Risiko

| Risiko | Dampak | Mitigasi |
|---|---|---|
| Kafka berat di VPS kecil | Biaya naik / OOM | Tune heap, atau ganti Redpanda (API kompatibel) |
| Admin kesulitan pakai panel | Kembali ke chat manual | UI sangat sederhana, uji ke orangtua sejak M2 |
| Harga/stok tidak diperbarui | Pesanan keliru | Pengingat harian, update stok cepat, validasi saat checkout |
| Bukti transfer palsu | Kerugian | Verifikasi manual admin + rekonsiliasi mutasi bank |
| Scope creep (fitur latihan) | Tidak selesai | Patuhi milestone; fitur baru masuk backlog |

---

## 10. Roadmap ringkas

| Milestone | Fokus | Estimasi |
|---|---|---|
| M0 | Fondasi repo, infra, skeleton service | 1–2 hari |
| M1 | Auth, gateway, kerangka frontend | 2–3 hari |
| M2 | Katalog + panel produk | 3–4 hari |
| M3 | Ongkir, keranjang, checkout, order + Kafka | 4–5 hari |
| M4 | Pembayaran, verifikasi, Telegram | 3–4 hari |
| M5 | Riwayat, lacak guest, expire, DLQ | 2–3 hari |
| M6 | Observability & hardening | 2 hari |
| M7 | Deploy VPS, CI/CD, backup | 2 hari |

---

## 11. Definisi "selesai" produk (v1)

Pembeli dapat menelusuri katalog, checkout sebagai guest, memilih ongkir, membayar transfer/COD,
mengunggah bukti, dan melacak status. Admin dapat mengelola produk, melihat pesanan, memverifikasi
pembayaran, dan menerima notifikasi Telegram. Semua berjalan di VPS dengan HTTPS dan backup harian.
