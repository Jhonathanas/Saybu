import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes, useParams } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useCart } from '@/features/cart/store'
import type { Product } from '@/features/catalog/types'
import { ApiError } from '@/lib/api'
import { CheckoutPage } from './CheckoutPage'
import type { Order, Quote, Zone } from './types'
import * as api from './api'

vi.mock('@/features/auth/AuthProvider', () => ({
  useAuth: () => ({ user: null, loading: false }),
}))

vi.mock('./api', async (importOriginal) => {
  const actual = await importOriginal<typeof import('./api')>()
  return {
    ...actual,
    listZones: vi.fn(),
    getQuote: vi.fn(),
    createOrder: vi.fn(),
    newIdempotencyKey: vi.fn(() => 'test-key'),
  }
})

const mocked = vi.mocked(api)

const ZONE: Zone = {
  id: 'zone-1',
  name: 'Kec. Sukamaju',
  areas: ['Sukamaju'],
  fee: 10000,
  free_min_subtotal: 150000,
}

const QUOTE: Quote = {
  zone_id: ZONE.id,
  zone_name: ZONE.name,
  fee: 10000,
  free_shipping: false,
  subtotal: 10000,
  total: 20000,
}

function order(): Order {
  return {
    id: 'o-1',
    code: 'SB-20261114-0001',
    status: 'PENDING_PAYMENT',
    fulfillment: 'DELIVERY',
    payment_method: 'TRANSFER',
    customer: { name: 'Siti', phone: '081234567890' },
    address: { line: 'Jl. Melati 12' },
    zone_id: ZONE.id,
    zone_name: ZONE.name,
    subtotal: 10000,
    shipping_fee: 10000,
    total: 20000,
    items: [],
    status_history: [],
    expires_at: '2026-11-15T03:00:00Z',
    created_at: '2026-11-14T03:00:00Z',
  }
}

function product(): Product {
  return {
    id: 'p-1',
    slug: 'bayam-segar',
    name: 'Bayam Segar',
    description: '',
    unit: 'ikat',
    price: 5000,
    stock: 40,
    min_qty: 1,
    step_qty: 1,
    is_available: true,
    images: [],
  }
}

function DetailStub() {
  const { code } = useParams()
  return <div>Detail {code}</div>
}

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter initialEntries={['/checkout']}>
        <Routes>
          <Route path="/checkout" element={<CheckoutPage />} />
          <Route path="/pesanan/:code" element={<DetailStub />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  useCart.getState().clear()
  mocked.listZones.mockResolvedValue([ZONE])
  mocked.getQuote.mockResolvedValue(QUOTE)
  mocked.newIdempotencyKey.mockReturnValue('test-key')
})

async function fillDeliveryForm() {
  await userEvent.type(screen.getByLabelText('Nama'), 'Siti')
  await userEvent.type(screen.getByLabelText('Nomor HP / WhatsApp'), '081234567890')
  await userEvent.selectOptions(await screen.findByLabelText('Zona ongkir'), ZONE.id)
  await userEvent.type(screen.getByLabelText('Alamat lengkap'), 'Jl. Melati 12')
}

describe('CheckoutPage', () => {
  it('menampilkan ajakan saat keranjang kosong', () => {
    renderPage()
    expect(screen.getByText(/Keranjang kosong/)).toBeInTheDocument()
  })

  it('menghitung total = subtotal + ongkir dari kuotasi', async () => {
    useCart.getState().add(product(), 2) // 2 x Rp5.000 = Rp10.000
    renderPage()
    await fillDeliveryForm()

    await waitFor(() => expect(mocked.getQuote).toHaveBeenCalledWith(ZONE.id, 10000))
    // Subtotal 10.000 + ongkir 10.000 = 20.000 (muncul di ringkasan & total).
    await waitFor(() => expect(screen.getAllByText(/Rp\s?20\.000/).length).toBeGreaterThanOrEqual(1))
  })

  it('menolak submit tanpa data wajib', async () => {
    useCart.getState().add(product(), 1)
    renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Buat pesanan' }))
    expect(await screen.findByText('Nama wajib diisi.')).toBeInTheDocument()
    expect(mocked.createOrder).not.toHaveBeenCalled()
  })

  it('mengirim checkout dengan header Idempotency-Key lalu pindah ke detail', async () => {
    useCart.getState().add(product(), 2)
    mocked.createOrder.mockResolvedValue(order())
    renderPage()
    await fillDeliveryForm()
    await userEvent.click(screen.getByRole('button', { name: 'Buat pesanan' }))

    await waitFor(() => expect(mocked.createOrder).toHaveBeenCalledTimes(1))
    const [payload, key] = mocked.createOrder.mock.calls[0]
    expect(key).toBe('test-key')
    expect(payload.items).toEqual([{ product_id: 'p-1', qty: 2 }])
    expect(payload.fulfillment).toBe('DELIVERY')
    expect(await screen.findByText('Detail SB-20261114-0001')).toBeInTheDocument()
    // Keranjang dikosongkan setelah pesanan dibuat.
    expect(useCart.getState().lines).toHaveLength(0)
  })

  it('menampilkan pesan saat stok tidak cukup (409)', async () => {
    useCart.getState().add(product(), 1)
    mocked.createOrder.mockRejectedValue(
      new ApiError(409, 'CONFLICT', 'Stok tidak cukup untuk sebagian produk.'),
    )
    renderPage()
    await fillDeliveryForm()
    await userEvent.click(screen.getByRole('button', { name: 'Buat pesanan' }))

    expect(
      await screen.findByText(/Stok tidak cukup untuk sebagian produk\./),
    ).toBeInTheDocument()
  })
})
