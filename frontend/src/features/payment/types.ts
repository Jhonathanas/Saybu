// Tipe pembayaran (selaras dengan docs/API_SPEC.md §6 dan
// backend/services/payment/internal/domain).

export type PaymentMethod = 'TRANSFER' | 'COD'

export type PaymentStatus =
  | 'UNPAID'
  | 'WAITING_VERIFICATION'
  | 'PAID'
  | 'REJECTED'
  | 'CANCELLED'
  | 'COD_PENDING'
  | 'COD_PAID'

export interface BankInfo {
  name: string
  account_number: string
  account_holder: string
}

/** Payment adalah status pembayaran sebuah pesanan. */
export interface Payment {
  method: PaymentMethod
  status: PaymentStatus
  amount: number
  /** Terisi hanya untuk metode TRANSFER. */
  bank?: BankInfo | null
  proof_url?: string | null
  verified_at?: string | null
  note?: string | null
}

/** AdminPayment adalah baris pembayaran untuk panel admin. */
export interface AdminPayment {
  id: string
  order_code: string
  customer_name: string
  customer_phone: string
  method: PaymentMethod
  status: PaymentStatus
  amount: number
  proof_url?: string | null
  uploaded_at?: string | null
  verified_at?: string | null
  note?: string | null
  created_at: string
}

export interface ListPaymentsMeta {
  page: number
  limit: number
  total: number
}

type PaymentTone = 'pending' | 'done' | 'error' | 'cancelled'

interface PaymentStatusInfo {
  label: string
  tone: PaymentTone
}

const PAYMENT_STATUS: Record<string, PaymentStatusInfo> = {
  UNPAID: { label: 'Belum dibayar', tone: 'pending' },
  WAITING_VERIFICATION: { label: 'Menunggu verifikasi', tone: 'pending' },
  PAID: { label: 'Sudah dibayar', tone: 'done' },
  REJECTED: { label: 'Bukti ditolak', tone: 'error' },
  CANCELLED: { label: 'Dibatalkan', tone: 'cancelled' },
  COD_PENDING: { label: 'Bayar di tempat', tone: 'pending' },
  COD_PAID: { label: 'Sudah dibayar', tone: 'done' },
}

export function paymentStatusLabel(status: string): string {
  return PAYMENT_STATUS[status]?.label ?? status
}

const TONE_CLASS: Record<PaymentTone, string> = {
  pending: 'bg-amber-50 text-amber-800 ring-amber-200',
  done: 'bg-brand-50 text-brand-800 ring-brand-200',
  error: 'bg-red-50 text-red-700 ring-red-200',
  cancelled: 'bg-stone-100 text-stone-600 ring-stone-200',
}

export function paymentStatusBadgeClass(status: string): string {
  return TONE_CLASS[PAYMENT_STATUS[status]?.tone ?? 'pending']
}

/** Status yang mengizinkan pelanggan mengunggah (ulang) bukti transfer. */
export function canUploadProof(status: string): boolean {
  return status === 'UNPAID' || status === 'REJECTED'
}
