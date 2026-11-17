// Tipe untuk ongkir & pesanan (selaras dengan docs/API_SPEC.md §4 & §5).

export type Fulfillment = 'DELIVERY' | 'PICKUP'
export type PaymentMethod = 'TRANSFER' | 'COD'

export interface Zone {
  id: string
  name: string
  areas: string[]
  fee: number
  free_min_subtotal: number
}

export interface Quote {
  zone_id: string
  zone_name: string
  fee: number
  free_shipping: boolean
  subtotal: number
  total: number
}

export interface OrderCustomer {
  name: string
  phone: string
  email?: string
  note?: string
}

export interface OrderAddress {
  line?: string
  rt_rw?: string
  kelurahan?: string
  kecamatan?: string
  city?: string
  postal_code?: string
  landmark?: string
}

export interface OrderItem {
  product_id: string
  name: string
  unit: string
  /** Jumlah dalam satuan terkecil (gram untuk kg). */
  qty: number
  /** Jumlah siap tampil, mis. "2 ikat" / "1 kg". */
  qty_display: string
  unit_price: number
  line_total: number
}

export interface StatusHistoryEntry {
  status: string
  note?: string
  at: string
}

export interface Order {
  id: string
  code: string
  status: string
  fulfillment: Fulfillment
  payment_method: PaymentMethod
  customer: OrderCustomer
  address: OrderAddress
  zone_id?: string
  zone_name?: string
  subtotal: number
  shipping_fee: number
  total: number
  items: OrderItem[]
  status_history: StatusHistoryEntry[]
  cancel_reason?: string
  expires_at: string
  created_at: string
}

export interface CreateOrderItemInput {
  product_id: string
  qty: number
}

export interface CreateOrderInput {
  customer: {
    name: string
    phone: string
    email?: string
    note?: string
  }
  fulfillment: Fulfillment
  zone_id?: string
  address?: OrderAddress
  payment_method: PaymentMethod
  items: CreateOrderItemInput[]
}

export interface ListOrdersMeta {
  page: number
  limit: number
  total: number
}

/** Status pesanan (selaras dengan backend domain/state.go). */
export type OrderStatus =
  | 'PENDING_PAYMENT'
  | 'WAITING_VERIFICATION'
  | 'PAID'
  | 'PREPARING'
  | 'SHIPPED'
  | 'READY_PICKUP'
  | 'COMPLETED'
  | 'CANCELLED'
  | 'EXPIRED'

/** Filter daftar pesanan admin (GET /admin/orders). */
export interface AdminOrdersQuery {
  status?: string
  /** YYYY-MM-DD, inklusif (zona Asia/Jakarta). */
  date_from?: string
  /** YYYY-MM-DD, inklusif. */
  date_to?: string
  q?: string
  page?: number
  limit?: number
}
