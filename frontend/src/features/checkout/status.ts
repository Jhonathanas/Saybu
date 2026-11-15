// Label & warna status pesanan untuk tampilan (selaras dengan
// backend/services/order/internal/domain/state.go).

export type StatusTone = 'pending' | 'progress' | 'done' | 'cancelled'

interface StatusInfo {
  label: string
  tone: StatusTone
}

const STATUS: Record<string, StatusInfo> = {
  PENDING_PAYMENT: { label: 'Menunggu Pembayaran', tone: 'pending' },
  WAITING_VERIFICATION: { label: 'Menunggu Verifikasi', tone: 'pending' },
  PAID: { label: 'Sudah Dibayar', tone: 'progress' },
  PREPARING: { label: 'Sedang Disiapkan', tone: 'progress' },
  SHIPPED: { label: 'Sedang Dikirim', tone: 'progress' },
  READY_PICKUP: { label: 'Siap Diambil', tone: 'progress' },
  COMPLETED: { label: 'Selesai', tone: 'done' },
  CANCELLED: { label: 'Dibatalkan', tone: 'cancelled' },
  EXPIRED: { label: 'Kedaluwarsa', tone: 'cancelled' },
}

export function statusLabel(status: string): string {
  return STATUS[status]?.label ?? status
}

export function statusTone(status: string): StatusTone {
  return STATUS[status]?.tone ?? 'pending'
}

const TONE_CLASS: Record<StatusTone, string> = {
  pending: 'bg-amber-50 text-amber-800 ring-amber-200',
  progress: 'bg-sky-50 text-sky-800 ring-sky-200',
  done: 'bg-brand-50 text-brand-800 ring-brand-200',
  cancelled: 'bg-stone-100 text-stone-600 ring-stone-200',
}

export function statusBadgeClass(status: string): string {
  return TONE_CLASS[statusTone(status)]
}

/** Status yang masih boleh dibatalkan pembeli (lihat domain.CancellableByCustomer). */
export function isCancellable(status: string): boolean {
  return status === 'PENDING_PAYMENT' || status === 'WAITING_VERIFICATION' || status === 'PAID'
}

/** Status yang menandakan pesanan masih menunggu pembayaran/verifikasi. */
export function isUnpaid(status: string): boolean {
  return status === 'PENDING_PAYMENT' || status === 'WAITING_VERIFICATION'
}

export function fulfillmentLabel(fulfillment: string): string {
  return fulfillment === 'PICKUP' ? 'Ambil di tempat' : 'Dikirim'
}

export function paymentLabel(method: string): string {
  return method === 'COD' ? 'Bayar di tempat (COD)' : 'Transfer bank'
}
