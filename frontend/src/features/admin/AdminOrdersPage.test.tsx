import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Order } from '@/features/checkout/types'
import * as checkoutApi from '@/features/checkout/api'
import { AdminOrdersPage } from './AdminOrdersPage'

vi.mock('@/features/checkout/api', () => ({
  adminListOrders: vi.fn(),
}))

const mocked = vi.mocked(checkoutApi)

const ORDER: Order = {
  id: 'o-1',
  code: 'SB-20261117-0001',
  status: 'WAITING_VERIFICATION',
  fulfillment: 'DELIVERY',
  payment_method: 'TRANSFER',
  customer: { name: 'Siti', phone: '081234567890' },
  address: { line: 'Jl. Melati 3' },
  subtotal: 70000,
  shipping_fee: 10000,
  total: 80000,
  items: [],
  status_history: [],
  expires_at: '2026-11-18T03:00:00Z',
  created_at: '2026-11-17T03:00:00Z',
}

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <AdminOrdersPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mocked.adminListOrders.mockResolvedValue({
    data: [ORDER],
    meta: { page: 1, limit: 20, total: 1 },
  })
})

describe('AdminOrdersPage', () => {
  it('menampilkan daftar pesanan dengan tautan detail', async () => {
    renderPage()

    const link = await screen.findByRole('link', { name: 'SB-20261117-0001' })
    expect(link).toHaveAttribute('href', '/admin/pesanan/o-1')
    expect(screen.getByText('Siti')).toBeInTheDocument()
    expect(screen.getByText(/Rp\s?80\.000/)).toBeInTheDocument()
    expect(mocked.adminListOrders).toHaveBeenCalledWith({
      status: undefined,
      date_from: undefined,
      date_to: undefined,
      q: undefined,
      page: 1,
      limit: 20,
    })
  })

  it('menampilkan pesan saat tidak ada pesanan', async () => {
    mocked.adminListOrders.mockResolvedValue({
      data: [],
      meta: { page: 1, limit: 20, total: 0 },
    })
    renderPage()

    expect(await screen.findByText('Tidak ada pesanan pada filter ini.')).toBeInTheDocument()
  })
})
