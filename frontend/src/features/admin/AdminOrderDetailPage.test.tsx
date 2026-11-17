import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter, Route, Routes } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Order } from '@/features/checkout/types'
import * as checkoutApi from '@/features/checkout/api'
import { AdminOrderDetailPage } from './AdminOrderDetailPage'

vi.mock('@/features/checkout/api', () => ({
  adminGetOrder: vi.fn(),
  adminUpdateOrderStatus: vi.fn(),
}))

const mocked = vi.mocked(checkoutApi)

const ORDER: Order = {
  id: 'o-1',
  code: 'SB-20261117-0001',
  status: 'PENDING_PAYMENT',
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
      <MemoryRouter initialEntries={['/admin/pesanan/o-1']}>
        <Routes>
          <Route path="/admin/pesanan/:id" element={<AdminOrderDetailPage />} />
        </Routes>
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mocked.adminGetOrder.mockResolvedValue(ORDER)
  mocked.adminUpdateOrderStatus.mockResolvedValue({ ...ORDER, status: 'PAID' })
})

describe('AdminOrderDetailPage', () => {
  it('menampilkan form ubah status dengan transisi yang sah', async () => {
    renderPage()

    expect(await screen.findByText('SB-20261117-0001')).toBeInTheDocument()
    const select = await screen.findByLabelText('Status baru')
    const values = Array.from(select.querySelectorAll('option')).map((o) => o.getAttribute('value'))
    expect(values).toEqual(['WAITING_VERIFICATION', 'PAID', 'CANCELLED', 'EXPIRED'])
  })

  it('mengirim perubahan status dengan catatan', async () => {
    renderPage()

    await userEvent.selectOptions(await screen.findByLabelText('Status baru'), 'PAID')
    await userEvent.type(screen.getByLabelText('Catatan (opsional)'), 'Transfer masuk')
    await userEvent.click(screen.getByRole('button', { name: 'Simpan status' }))

    await waitFor(() =>
      expect(mocked.adminUpdateOrderStatus).toHaveBeenCalledWith('o-1', 'PAID', 'Transfer masuk'),
    )
  })

  it('menandai status final tanpa opsi perubahan', async () => {
    mocked.adminGetOrder.mockResolvedValue({ ...ORDER, status: 'COMPLETED' })
    renderPage()

    expect(await screen.findByText(/bersifat final/)).toBeInTheDocument()
    expect(screen.queryByLabelText('Status baru')).not.toBeInTheDocument()
  })
})
