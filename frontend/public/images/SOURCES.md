# Sumber aset gambar

Semua foto di folder ini berasal dari [Unsplash](https://unsplash.com/license) (lisensi bebas
untuk penggunaan komersial, atribusi tidak wajib tetapi dicantumkan di sini). Gambar diunduh
langsung sebagai WebP dari CDN `images.unsplash.com` dengan parameter `w`, `q`, `fm=webp`, dan
`fit=crop` agar ukuran file tetap kecil (target < 100 KB per file).

| File | Sumber | Parameter |
|---|---|---|
| `hero-sayur.webp` | https://images.unsplash.com/photo-1542838132-92c53300491e | `w=600&q=58` |
| `kategori-semua.webp` | https://images.unsplash.com/photo-1542838132-92c53300491e | `w=200&q=65` |
| `kategori-buah.webp` | https://images.unsplash.com/photo-1769455538006-61d321fcf782 | `w=200&q=65` |
| `kategori-sayur.webp` | https://images.unsplash.com/photo-1788710320160-fd1661aa6c54 | `w=200&q=65` |
| `kategori-hemat.webp` | https://images.unsplash.com/photo-1763844668879-528c9dafa402 | `w=200&q=65` |
| `produk-default.webp` | https://images.unsplash.com/photo-1542838132-92c53300491e | `w=200&q=65` (salinan `kategori-semua.webp`, dipakai bila foto produk kosong/rusak) |
| `produk-pisang.webp` | https://images.unsplash.com/photo-1757332050958-b797a022c910 | `w=320&q=70` |
| `produk-apel.webp` | https://images.unsplash.com/photo-1568702846914-96b305d2aaeb | `w=320&q=70` |
| `produk-bayam.webp` | https://images.unsplash.com/photo-1574316071802-0d684efa7bf5 | `w=320&q=70` |
| `produk-tomat.webp` | https://images.unsplash.com/photo-1745791562822-7ac21012bbb2 | `w=320&q=70` |

Foto produk di atas hanya dipakai sebagai contoh/tampilan katalog. Data harga, stok, dan gambar
produk asli tetap berasal dari API (lihat `AGENTS.md`).
