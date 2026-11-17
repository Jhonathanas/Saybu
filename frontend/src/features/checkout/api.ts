import { request, requestWithMeta } from '@/lib/api'
import type {
  AdminOrdersQuery,
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

// --- admin ---

/** adminListOrders mengambil daftar pesanan dengan filter & paginasi (admin). */
export function adminListOrders(
  params: AdminOrdersQuery = {},
): Promise<{ data: Order[]; meta?: ListOrdersMeta }> {
  const query = new URLSearchParams()
  if (params.status) query.set('status', params.status)
  if (params.date_from) query.set('date_from', params.date_from)
  if (params.date_to) query.set('date_to', params.date_to)
  if (params.q) query.set('q', params.q)
  if (params.page) query.set('page', String(params.page))
  if (params.limit) query.set('limit', String(params.limit))
  const suffix = query.toString() ? `?${query.toString()}` : ''
  return requestWithMeta<Order[], ListOrdersMeta>(`/admin/orders${suffix}`)
}

/** adminGetOrder mengambil satu pesanan berdasarkan id (admin). */
export function adminGetOrder(id: string): Promise<Order> {
  return request<Order>(`/admin/orders/${encodeURIComponent(id)}`)
}

/** adminUpdateOrderStatus mengubah status pesanan (admin). 409 bila ilegal. */
export function adminUpdateOrderStatus(id: string, status: string, note = ''): Promise<Order> {
  return request<Order>(`/admin/orders/${encodeURIComponent(id)}/status`, {
    method: 'PATCH',
    body: { status, note },
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
