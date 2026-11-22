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

/** Nada status memakai token shadcn/ui agar konsisten di mode terang & gelap. */
const TONE_CLASS: Record<StatusTone, string> = {
  pending: 'bg-muted text-foreground ring-border',
  progress: 'bg-secondary text-secondary-foreground ring-border',
  done: 'bg-foreground text-background ring-foreground',
  cancelled: 'bg-background text-muted-foreground ring-border',
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

/**
 * Transisi status yang sah, selaras dengan backend domain/state.go.
 * Dipakai panel admin agar pilihan status selalu legal.
 */
const TRANSITIONS: Record<string, string[]> = {
  PENDING_PAYMENT: ['WAITING_VERIFICATION', 'PAID', 'CANCELLED', 'EXPIRED'],
  WAITING_VERIFICATION: ['PAID', 'PENDING_PAYMENT', 'CANCELLED'],
  PAID: ['PREPARING', 'CANCELLED'],
  PREPARING: ['SHIPPED', 'READY_PICKUP', 'CANCELLED'],
  SHIPPED: ['COMPLETED'],
  READY_PICKUP: ['COMPLETED'],
  COMPLETED: [],
  CANCELLED: [],
  EXPIRED: [],
}

/** allowedTransitions mengembalikan status tujuan yang sah dari status saat ini. */
export function allowedTransitions(status: string): string[] {
  return TRANSITIONS[status] ?? []
}
