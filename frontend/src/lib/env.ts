// Konfigurasi frontend dari environment Vite.
// Nilai default dipakai saat `.env` tidak ada (dev lokal).

export const API_BASE_URL = import.meta.env.VITE_API_BASE_URL ?? '/api/v1'

export const APP_NAME = 'Saybu'

export const isProduction = import.meta.env.PROD
