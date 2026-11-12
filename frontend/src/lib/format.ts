// Helper format uang & tanggal untuk locale Indonesia (Asia/Jakarta).

const currency = new Intl.NumberFormat('id-ID', {
  style: 'currency',
  currency: 'IDR',
  minimumFractionDigits: 0,
  maximumFractionDigits: 0,
})

const dateTime = new Intl.DateTimeFormat('id-ID', {
  dateStyle: 'medium',
  timeStyle: 'short',
  timeZone: 'Asia/Jakarta',
})

const dateOnly = new Intl.DateTimeFormat('id-ID', {
  dateStyle: 'long',
  timeZone: 'Asia/Jakarta',
})

/** Format rupiah dari nilai integer (tanpa desimal). */
export function formatIDR(value: number): string {
  if (!Number.isFinite(value)) return '—'
  return currency.format(value)
}

/** Format waktu lengkap dalam zona Asia/Jakarta. */
export function formatDateTime(value: string | number | Date): string {
  const date = toDate(value)
  return date ? dateTime.format(date) : '—'
}

/** Format tanggal (tanpa jam) dalam zona Asia/Jakarta. */
export function formatDate(value: string | number | Date): string {
  const date = toDate(value)
  return date ? dateOnly.format(date) : '—'
}

function toDate(value: string | number | Date): Date | null {
  const date = value instanceof Date ? value : new Date(value)
  return Number.isNaN(date.getTime()) ? null : date
}
