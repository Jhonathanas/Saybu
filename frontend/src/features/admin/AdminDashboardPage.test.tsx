import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/features/admin/api', () => ({
  getTodayStats: vi.fn(),
}))

vi.mock('@/features/catalog/api', () => ({
  adminListProducts: vi.fn(),
}))

import { getTodayStats } from '@/features/admin/api'
import { adminListProducts } from '@/features/catalog/api'
import { AdminDashboardPage } from './AdminDashboardPage'

const mockedStats = vi.mocked(getTodayStats)
const mockedProducts = vi.mocked(adminListProducts)

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <AdminDashboardPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockedStats.mockResolvedValue({
    orders_count: 4,
    revenue_paid: 320000,
    awaiting_verification: 2,
    low_stock: [{ product_id: 'p-1', name: 'Bayam', stock: 5, unit: 'ikat' }],
  })
  mockedProducts.mockResolvedValue({
    data: [],
    meta: { page: 1, limit: 1, total: 7, total_pages: 7 },
  })
})

describe('AdminDashboardPage', () => {
  it('menampilkan ringkasan hari ini dari API', async () => {
    renderPage()

    expect(await screen.findByText('4')).toBeInTheDocument()
    expect(screen.getByText(/Rp\s?320\.000/)).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
    expect(await screen.findByText('7')).toBeInTheDocument()
  })

  it('menampilkan produk dengan stok menipis', async () => {
    renderPage()

    expect(await screen.findByText('Bayam')).toBeInTheDocument()
    expect(screen.getByText(/Tersisa 5 ikat/)).toBeInTheDocument()
  })

  it('menampilkan pesan aman saat stok cukup', async () => {
    mockedStats.mockResolvedValue({
      orders_count: 0,
      revenue_paid: 0,
      awaiting_verification: 0,
      low_stock: [],
    })
    renderPage()

    expect(await screen.findByText('Semua stok masih aman.')).toBeInTheDocument()
  })
})
