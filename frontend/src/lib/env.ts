// Konfigurasi frontend dari environment Vite.
// Nilai default dipakai saat `.env` tidak ada (dev lokal).

export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

export const APP_NAME = 'Saybu'

export const isProduction = import.meta.env.PROD

// Instruksi transfer manual ditampilkan di halaman sukses checkout.
// Sumber nilai: `frontend/.env` (VITE_BANK_*), selaras dengan `BANK_*` di `.env` backend.
export const BANK = {
  name: import.meta.env.VITE_BANK_NAME ?? 'BCA',
  account: import.meta.env.VITE_BANK_ACCOUNT ?? '1234567890',
  holder: import.meta.env.VITE_BANK_HOLDER ?? 'Saybu Buah & Sayur',
} as const
