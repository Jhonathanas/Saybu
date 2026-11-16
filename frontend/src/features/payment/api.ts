// Panggilan API pembayaran: status pelanggan, unggah bukti, dan verifikasi admin.
import { request, requestWithMeta, uploadFile } from '@/lib/api'
import type { AdminPayment, ListPaymentsMeta, Payment } from './types'

/** getPayment mengambil status pembayaran sebuah pesanan (guest pakai phone). */
export function getPayment(code: string, phone?: string): Promise<Payment> {
  const query = phone ? `?phone=${encodeURIComponent(phone)}` : ''
  return request<Payment>(`/orders/${encodeURIComponent(code)}/payment${query}`)
}

/** uploadPaymentProof mengunggah bukti transfer (JPG/PNG/PDF, maks 5 MB). */
export function uploadPaymentProof(code: string, file: File, phone?: string): Promise<Payment> {
  return uploadFile<Payment>(
    `/orders/${encodeURIComponent(code)}/payment-proof`,
    file,
    phone ? { phone } : undefined,
  )
}

/** listPayments mengambil daftar pembayaran untuk panel admin. */
export function listPayments(
  params: { status?: string; page?: number; limit?: number } = {},
): Promise<{ data: AdminPayment[]; meta?: ListPaymentsMeta }> {
  const query = new URLSearchParams()
  if (params.status) query.set('status', params.status)
  if (params.page) query.set('page', String(params.page))
  if (params.limit) query.set('limit', String(params.limit))
  const suffix = query.toString() ? `?${query.toString()}` : ''
  return requestWithMeta<AdminPayment[], ListPaymentsMeta>(`/admin/payments${suffix}`)
}

/** verifyPayment menyetujui atau menolak bukti transfer (admin). */
export function verifyPayment(
  id: string,
  decision: 'APPROVE' | 'REJECT',
  note = '',
): Promise<AdminPayment> {
  return request<AdminPayment>(`/admin/payments/${encodeURIComponent(id)}/verify`, {
    method: 'POST',
    body: { decision, note },
  })
}
