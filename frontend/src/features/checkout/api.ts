import { request, requestWithMeta } from '@/lib/api'
import type {
  CreateOrderInput,
  ListOrdersMeta,
  Order,
  Quote,
  Zone,
} from './types'

/** listZones mengambil zona ongkir aktif (publik). */
export function listZones(): Promise<Zone[]> {
  return request<Zone[]>('/shipping/zones', { auth: false })
}

/** getQuote menghitung ongkir untuk zona & subtotal tertentu. */
export function getQuote(zoneId: string, subtotal: number): Promise<Quote> {
  const query = new URLSearchParams({ zone_id: zoneId, subtotal: String(subtotal) })
  return request<Quote>(`/shipping/quote?${query.toString()}`, { auth: false })
}

/**
 * createOrder mengirim checkout. `idempotencyKey` wajib agar pengiriman ulang
 * (mis. klik ganda / retry jaringan) tidak membuat pesanan ganda.
 */
export function createOrder(input: CreateOrderInput, idempotencyKey: string): Promise<Order> {
  return request<Order>('/orders', {
    method: 'POST',
    body: input,
    headers: { 'Idempotency-Key': idempotencyKey },
  })
}

/** getOrder melacak pesanan. Guest wajib menyertakan nomor HP. */
export function getOrder(code: string, phone?: string): Promise<Order> {
  const query = phone ? `?phone=${encodeURIComponent(phone)}` : ''
  return request<Order>(`/orders/${encodeURIComponent(code)}${query}`)
}

/** listMyOrders mengambil riwayat pesanan milik user yang login. */
export function listMyOrders(params: { page?: number; limit?: number } = {}) {
  const query = new URLSearchParams()
  if (params.page) query.set('page', String(params.page))
  if (params.limit) query.set('limit', String(params.limit))
  const suffix = query.toString() ? `?${query.toString()}` : ''
  return requestWithMeta<Order[], ListOrdersMeta>(`/me/orders${suffix}`)
}

/** cancelOrder membatalkan pesanan (guest menyertakan nomor HP). */
export function cancelOrder(code: string, phone?: string, reason?: string): Promise<Order> {
  return request<Order>(`/orders/${encodeURIComponent(code)}/cancel`, {
    method: 'POST',
    body: { phone: phone ?? '', reason: reason ?? '' },
  })
}

/**
 * newIdempotencyKey membuat kunci idempoten untuk checkout. Memakai
 * `crypto.randomUUID` bila tersedia, dengan fallback untuk lingkungan lama.
 */
export function newIdempotencyKey(): string {
  if (typeof crypto !== 'undefined' && typeof crypto.randomUUID === 'function') {
    return crypto.randomUUID()
  }
  return `idem-${Date.now()}-${Math.random().toString(16).slice(2)}`
}
