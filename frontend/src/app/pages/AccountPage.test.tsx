import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Order } from '@/features/checkout/types'
import type { User } from '@/features/auth/types'

vi.mock('@/features/auth/AuthProvider', () => ({
  useAuth: vi.fn(),
}))

vi.mock('@/features/checkout/api', () => ({
  listMyOrders: vi.fn(),
}))

import { useAuth } from '@/features/auth/AuthProvider'
import * as checkoutApi from '@/features/checkout/api'
import { AccountPage } from './AccountPage'

const mockedAuth = vi.mocked(useAuth)
const mockedApi = vi.mocked(checkoutApi)

const USER: User = {
  id: 'u-1',
  name: 'Sari',
  phone: '081234567890',
  email: 'sari@example.com',
  role: 'customer',
}

const ORDER: Order = {
  id: 'o-1',
  code: 'SB-20261117-0002',
  status: 'SHIPPED',
  fulfillment: 'DELIVERY',
  payment_method: 'TRANSFER',
  customer: { name: 'Sari', phone: '081234567890' },
  address: { line: 'Jl. Kenanga 1' },
  subtotal: 40000,
  shipping_fee: 12000,
  total: 52000,
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
        <AccountPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockedAuth.mockReturnValue({
    user: USER,
    loading: false,
    login: vi.fn(),
    register: vi.fn(),
    logout: vi.fn(),
    refreshUser: vi.fn(),
  })
  mockedApi.listMyOrders.mockResolvedValue({
    data: [ORDER],
    meta: { page: 1, limit: 10, total: 1 },
  })
})

describe('AccountPage', () => {
  it('menampilkan profil dan riwayat pesanan milik user', async () => {
    renderPage()

    expect(screen.getByText('Sari')).toBeInTheDocument()
    const link = await screen.findByRole('link', { name: 'SB-20261117-0002' })
    expect(link).toHaveAttribute('href', '/pesanan/SB-20261117-0002')
    expect(mockedApi.listMyOrders).toHaveBeenCalledWith({ page: 1, limit: 10 })
  })

  it('menampilkan ajakan belanja saat belum ada pesanan', async () => {
    mockedApi.listMyOrders.mockResolvedValue({
      data: [],
      meta: { page: 1, limit: 10, total: 0 },
    })
    renderPage()

    expect(await screen.findByText(/Belum ada pesanan/)).toBeInTheDocument()
  })
})
