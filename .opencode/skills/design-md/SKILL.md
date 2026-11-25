---
name: design-md
description: >
  Referensi bahasa visual untuk UI Saybu berdasarkan DESIGN.md (Vercel, Linear, Cal.com).
  Pakai saat membuat/mengubah halaman, komponen, layout, tipografi, warna, atau memberi
  masukan desain frontend. Termasuk panduan menerjemahkan gaya brand ke token shadcn/ui
  netral yang sudah dipakai Saybu. Jangan dipakai untuk logika backend, kontrak API, atau migrasi.
argument-hint: "[brand|halaman|pertanyaan, mis. 'vercel checkout', 'cal', 'buat kartu produk ala linear']"
---

# DESIGN.md — referensi visual Saybu

Skill ini membawa tiga spesifikasi visual dari
[awesome-design-md](https://github.com/VoltAgent/awesome-design-md) sebagai referensi:
**Vercel** (default), **Linear**, dan **Cal.com**. Semuanya netral/minimal dan cocok
dengan tema shadcn/ui Saybu yang sudah ada.

## Prioritas (jangan diabaikan)

1. `AGENTS.md` dan `docs/` di root repo adalah sumber kebenaran aturan kerja.
2. Token shadcn di `frontend/src/styles/index.css` adalah sumber kebenaran warna/spacing UI.
3. File di `references/` hanya **panduan visual**. Jangan menimpa CSS variables,
   mengganti font, atau mengubah kontrak API tanpa permintaan eksplisit dan persetujuan.

## Cara pakai

- Tanpa argumen → gunakan **Vercel** sebagai baseline.
- `/design-md vercel`, `/design-md linear`, `/design-md cal` → pilih referensi.
- `/design-md cal halaman checkout` → baca referensi Cal, lalu terapkan pada halaman checkout.
- Bila diminta "rebrand penuh" atau "copy persis brand ini", tanyakan dulu apakah
  token shadcn boleh diubah.

## Alur kerja

1. Baca `references/<brand>.md` yang relevan **sebelum** menyentuh kode.
2. Ambil prinsip yang masih kompatibel: hierarki tipografi, ritme spacing,
   hairline border, shadow bertumpuk, perilaku responsif, dan guardrail Do/Don't.
3. Terjemahkan ke implementasi Saybu yang sudah ada:
   - Komponen: `src/components/ui/shadcn/` (kanonik) + adapter di `src/components/ui/`.
   - Warna: token `--background`, `--foreground`, `--primary`, `--secondary`,
     `--muted`, `--accent`, `--destructive`, `--border`, `--ring`.
   - Font: `Geist Variable` sudah terpasang; jangan tambah font brand lain.
   - Radius & spacing: pakai skala Tailwind/shadcn yang sudah ada, bukan angka
     mentah dari DESIGN.md kecuali disetujui.
4. Pertahankan aturan produk: bahasa Indonesia, mobile-first, target sentuh ≥ 44 px,
   harga/qty/ongkir dihitung server, snapshot pesanan tidak diubah frontend.
5. Tambah/perbarui test RTL bila ada perubahan interaksi; jalankan
   `npm run lint && npm run typecheck && npm test` sebelum selesai.

## Mapping cepat (Vercel → Saybu)

| DESIGN.md | Token / kelas Saybu | Catatan |
|---|---|---|
| `colors.ink` / `primary` `#171717` | `bg-primary` / `text-foreground` | CTA utama sudah memakai ini |
| `colors.canvas` | `bg-card` | Permukaan kartu |
| `colors.canvas-soft` | `bg-muted` | Latar sekunder |
| `colors.hairline` | `border-border` / `ring-border` | Garis 1 px |
| `colors.body` / `mute` | `text-muted-foreground` | Teks sekunder |
| `colors.error` | `text-destructive` / `bg-destructive/10` | Error & validasi |
| `typography.display-*` weight 600 | `text-2xl`/`text-3xl` + `font-semibold` | Tracking negatif opsional, jangan berlebihan |
| `rounded.pill` 100 px CTA marketing | `rounded-lg` + `tap-target` | **Jangan** dipakai di tombol aksi produk |

## Guardrail khusus

- Mesh gradient multi-warna Vercel **hanya** boleh dipakai sebagai latar hero dan
  harus tetap memakai token netral Saybu (lihat implementasi `HomePage.tsx`).
  Jangan mengecilnya jadi badge/icon atau memotong jadi satu warna.
- Linear memakai aksen lavender `#5e6ad2`; Cal memakai navy. Keduanya **tidak**
  menggantikan `--primary` Saybu tanpa persetujuan.
- Jangan menghapus adapter UI lama (`Button.tsx`, `Card.tsx`, `Input.tsx`,
  `Select.tsx`, `Feedback.tsx`, `CopyButton.tsx`, `BottomNav.tsx`).
- `Select` sengaja native agar test `userEvent.selectOptions` tetap bekerja.

## Sumber

Lihat `SOURCES.md` untuk URL upstream, lisensi MIT, dan tanggal unduh.
