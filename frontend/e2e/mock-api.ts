import type { Page, Route } from '@playwright/test'

// Fixture API untuk E2E. Harga/qty "server" sengaja didefinisikan di sini
// (bukan dibaca dari request) agar test memastikan klien mengirim product_id
// dan qty, bukan harga final dari browser.

export const ORDER_CODE = 'SB-20261126-0001'
const ZONE_ID = 'z-1'
const ZONE_NAME = 'Kec. Sukamaju'
const SHIPPING_FEE = 10_000

interface Product {
  id: string
  slug: string
  name: string
  category: { id: string; name: string; slug: string; sort: number }
  description: string
  unit: string
  price: number
  stock: number
  min_qty: number
  step_qty: number
  is_available: boolean
  images: string[]
  created_at: string
}

interface CreateOrderInput {
  customer: { name: string; phone: string; email?: string; note?: string }
  fulfillment: 'DELIVERY' | 'PICKUP'
  zone_id?: string
  address?: Record<string, string>
  payment_method: 'TRANSFER' | 'COD'
  items: Array<{ product_id: string; qty: number }>
}

interface Order {
  id: string
  code: string
  status: string
  fulfillment: 'DELIVERY' | 'PICKUP'
  payment_method: 'TRANSFER' | 'COD'
  customer: { name: string; phone: string; email?: string; note?: string }
  address: Record<string, string>
  zone_id?: string
  zone_name?: string
  subtotal: number
  shipping_fee: number
  total: number
  items: Array<{
    product_id: string
    name: string
    unit: string
    qty: number
    qty_display: string
    unit_price: number
    line_total: number
  }>
  status_history: Array<{ status: string; at: string }>
  expires_at: string
  created_at: string
}

export interface MockState {
  /** Pesanan terakhir yang dibuat lewat POST /orders. */
  order?: Order
  createRequests: Array<{ body: CreateOrderInput; idempotencyKey: string | null }>
}

const categories = [
  { id: 'c-1', name: 'Buah', slug: 'buah', sort: 1 },
  { id: 'c-2', name: 'Sayur', slug: 'sayur', sort: 2 },
]

const products: Product[] = [
  {
    id: 'p-7',
    slug: 'bayam-segar',
    name: 'Bayam Segar',
    category: categories[1],
    description: 'Bayam segar dipanen pagi ini.',
    unit: 'ikat',
    price: 5_000,
    stock: 40,
    min_qty: 1,
    step_qty: 1,
    is_available: true,
    images: ['/images/produk-bayam.webp'],
    created_at: '2026-11-18T01:00:00Z',
  },
  {
    id: 'p-1',
    slug: 'pisang-cavendish',
    name: 'Pisang Cavendish',
    category: categories[0],
    description: 'Pisang cavendish matang merata.',
    unit: 'kg',
    price: 18_000,
    stock: 250,
    min_qty: 250,
    step_qty: 250,
    is_available: true,
    images: ['/images/produk-pisang.webp'],
    created_at: '2026-11-17T01:00:00Z',
  },
]

/** sampleOrder dipakai skenario lacak pesanan (tanpa checkout). */
export function sampleOrder(): Order {
  return {
    id: 'o-e2e-1',
    code: ORDER_CODE,
    status: 'PENDING_PAYMENT',
    fulfillment: 'DELIVERY',
    payment_method: 'TRANSFER',
    customer: { name: 'Siti Aminah', phone: '081234567890' },
    address: { line: 'Jl. Melati No. 12', city: 'Bandung' },
    zone_id: ZONE_ID,
    zone_name: ZONE_NAME,
    subtotal: 5_000,
    shipping_fee: SHIPPING_FEE,
    total: 15_000,
    items: [
      {
        product_id: 'p-7',
        name: 'Bayam Segar',
        unit: 'ikat',
        qty: 1,
        qty_display: '1 ikat',
        unit_price: 5_000,
        line_total: 5_000,
      },
    ],
    status_history: [{ status: 'PENDING_PAYMENT', at: '2026-11-26T01:00:00Z' }],
    expires_at: '2026-11-27T01:00:00Z',
    created_at: '2026-11-26T01:00:00Z',
  }
}

function json(route: Route, body: unknown, status = 200) {
  return route.fulfill({ status, contentType: 'application/json', body: JSON.stringify(body) })
}

function notFound(route: Route) {
  return json(route, { error: { code: 'NOT_FOUND', message: 'Tidak ditemukan' } }, 404)
}

function feeFor(subtotal: number): number {
  return subtotal >= 200_000 ? 0 : SHIPPING_FEE
}

/** mockApi memasang semua endpoint yang dipakai alur belanja. */
export async function mockApi(
  page: Page,
  options: { existingOrder?: boolean } = {},
): Promise<MockState> {
  const state: MockState = {
    order: options.existingOrder ? sampleOrder() : undefined,
    createRequests: [],
  }

  await page.route('**/api/v1/**', async (route) => {
    const method = route.request().method()
    const url = new URL(route.request().url())
    const path = url.pathname

    if (path === '/api/v1/auth/refresh') {
      return json(route, { error: { code: 'UNAUTHORIZED', message: 'Belum masuk' } }, 401)
    }

    if (path === '/api/v1/categories' && method === 'GET') {
      return json(route, { data: categories })
    }

    if (path === '/api/v1/products' && method === 'GET') {
      const category = url.searchParams.get('category')
      const q = url.searchParams.get('q')?.toLowerCase() ?? ''
      const sort = url.searchParams.get('sort') ?? 'newest'
      const pageNo = Number(url.searchParams.get('page') ?? '1') || 1
      const limit = Number(url.searchParams.get('limit') ?? '12') || 12

      let items = products.filter((p) => {
        if (category && p.category.slug !== category) return false
        if (q && !p.name.toLowerCase().includes(q)) return false
        return true
      })
      if (sort === 'price_asc') items = [...items].sort((a, b) => a.price - b.price)
      if (sort === 'price_desc') items = [...items].sort((a, b) => b.price - a.price)
      if (sort === 'newest') items = [...items].reverse()

      const total = items.length
      return json(route, {
        data: items.slice((pageNo - 1) * limit, pageNo * limit),
        meta: { page: pageNo, limit, total, total_pages: Math.max(1, Math.ceil(total / limit)) },
      })
    }

    const productMatch = path.match(/^\/api\/v1\/products\/([^/]+)$/)
    if (productMatch && method === 'GET') {
      const product = products.find((p) => p.slug === decodeURIComponent(productMatch[1]))
      return product ? json(route, { data: product }) : notFound(route)
    }

    if (path === '/api/v1/shipping/zones' && method === 'GET') {
      return json(route, {
        data: [
          {
            id: ZONE_ID,
            name: ZONE_NAME,
            areas: ['Sukamaju'],
            fee: SHIPPING_FEE,
            free_min_subtotal: 200_000,
          },
        ],
      })
    }

    if (path === '/api/v1/shipping/quote' && method === 'GET') {
      const zoneId = url.searchParams.get('zone_id') ?? ''
      const subtotal = Number(url.searchParams.get('subtotal') ?? '0')
      if (zoneId !== ZONE_ID) {
        return json(route, { error: { code: 'INVALID_ARGUMENT', message: 'Zona tidak dikenal' } }, 422)
      }
      const fee = feeFor(subtotal)
      return json(route, {
        data: {
          zone_id: zoneId,
          zone_name: ZONE_NAME,
          fee,
          free_shipping: fee === 0,
          subtotal,
          total: subtotal + fee,
        },
      })
    }

    if (path === '/api/v1/orders' && method === 'POST') {
      const body = route.request().postDataJSON() as CreateOrderInput
      const idempotencyKey = route.request().headers()['idempotency-key'] ?? null
      state.createRequests.push({ body, idempotencyKey })

      // Harga dihitung ulang dari fixture, bukan dari payload browser.
      const items = body.items.map((line) => {
        const product = products.find((p) => p.id === line.product_id)
        if (!product) return null
        return {
          product_id: product.id,
          name: product.name,
          unit: product.unit,
          qty: line.qty,
          qty_display: `${line.qty} ${product.unit}`,
          unit_price: product.price,
          line_total: product.price * line.qty,
        }
      })
      if (items.some((item) => item === null)) {
        return json(route, { error: { code: 'INVALID_ARGUMENT', message: 'Produk tidak dikenal' } }, 422)
      }
      const validItems = items.filter((item): item is NonNullable<typeof item> => item !== null)
      const subtotal = validItems.reduce((sum, item) => sum + item.line_total, 0)
      const shippingFee = body.fulfillment === 'DELIVERY' ? feeFor(subtotal) : 0
      const order: Order = {
        id: 'o-e2e-created',
        code: ORDER_CODE,
        status: 'PENDING_PAYMENT',
        fulfillment: body.fulfillment,
        payment_method: body.payment_method,
        customer: body.customer,
        address: body.address ?? {},
        zone_id: body.zone_id,
        zone_name: body.zone_id === ZONE_ID ? ZONE_NAME : undefined,
        subtotal,
        shipping_fee: shippingFee,
        total: subtotal + shippingFee,
        items: validItems,
        status_history: [{ status: 'PENDING_PAYMENT', at: '2026-11-26T01:00:00Z' }],
        expires_at: '2026-11-27T01:00:00Z',
        created_at: '2026-11-26T01:00:00Z',
      }
      state.order = order
      return json(route, { data: order }, 201)
    }

    const paymentMatch = path.match(/^\/api\/v1\/orders\/([^/]+)\/payment$/)
    if (paymentMatch && method === 'GET') {
      const order = state.order
      if (!order || decodeURIComponent(paymentMatch[1]) !== order.code) return notFound(route)
      return json(route, {
        data:
          order.payment_method === 'COD'
            ? { method: 'COD', status: 'COD_PENDING', amount: order.total }
            : {
                method: 'TRANSFER',
                status: 'UNPAID',
                amount: order.total,
                bank: {
                  name: 'BCA',
                  account_number: '1234567890',
                  account_holder: 'Saybu Buah & Sayur',
                },
              },
      })
    }

    const cancelMatch = path.match(/^\/api\/v1\/orders\/([^/]+)\/cancel$/)
    if (cancelMatch && method === 'POST') {
      const order = state.order
      if (!order || decodeURIComponent(cancelMatch[1]) !== order.code) return notFound(route)
      state.order = { ...order, status: 'CANCELLED', cancel_reason: 'Dibatalkan oleh pembeli' }
      return json(route, { data: state.order })
    }

    const orderMatch = path.match(/^\/api\/v1\/orders\/([^/]+)$/)
    if (orderMatch && method === 'GET') {
      const order = state.order
      if (!order || decodeURIComponent(orderMatch[1]) !== order.code) return notFound(route)
      return json(route, { data: order })
    }

    return notFound(route)
  })

  return state
}
