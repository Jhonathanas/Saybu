import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AdminPayment } from '@/features/payment/types'
import * as paymentApi from '@/features/payment/api'
import { AdminPaymentsPage } from './AdminPaymentsPage'

vi.mock('@/features/payment/api', () => ({
  listPayments: vi.fn(),
  verifyPayment: vi.fn(),
}))

const mocked = vi.mocked(paymentApi)

const PAYMENT: AdminPayment = {
  id: 'pay-1',
  order_code: 'SB-20261114-0001',
  customer_name: 'Siti',
  customer_phone: '081234567890',
  method: 'TRANSFER',
  status: 'WAITING_VERIFICATION',
  amount: 98000,
  proof_url: '/uploads/bukti.jpg',
  uploaded_at: '2026-11-14T04:00:00Z',
  verified_at: null,
  note: null,
  created_at: '2026-11-14T03:00:00Z',
}

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <AdminPaymentsPage />
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mocked.listPayments.mockResolvedValue({
    data: [PAYMENT],
    meta: { page: 1, limit: 20, total: 1 },
  })
})

describe('AdminPaymentsPage', () => {
  it('menampilkan daftar pembayaran menunggu verifikasi', async () => {
    renderPage()

    expect(await screen.findByText('SB-20261114-0001')).toBeInTheDocument()
    expect(screen.getByText('Siti')).toBeInTheDocument()
    expect(screen.getByText(/Rp\s?98\.000/)).toBeInTheDocument()
    expect(screen.getByRole('link', { name: 'Lihat bukti' })).toHaveAttribute(
      'href',
      '/uploads/bukti.jpg',
    )
    expect(mocked.listPayments).toHaveBeenCalledWith({
      status: 'WAITING_VERIFICATION',
      page: 1,
      limit: 20,
    })
  })

  it('menyetujui pembayaran', async () => {
    mocked.verifyPayment.mockResolvedValue({ ...PAYMENT, status: 'PAID' })
    renderPage()

    await userEvent.click(await screen.findByRole('button', { name: 'Setujui' }))
    await waitFor(() => expect(mocked.verifyPayment).toHaveBeenCalledWith('pay-1', 'APPROVE', ''))
  })

  it('menolak pembayaran dengan alasan', async () => {
    mocked.verifyPayment.mockResolvedValue({ ...PAYMENT, status: 'REJECTED', note: 'Bukti buram' })
    renderPage()

    await userEvent.click(await screen.findByRole('button', { name: 'Tolak' }))
    const reason = await screen.findByLabelText('Alasan penolakan (dikirim ke pelanggan)')
    await userEvent.type(reason, 'Bukti buram')
    await userEvent.click(screen.getByRole('button', { name: 'Kirim penolakan' }))

    await waitFor(() =>
      expect(mocked.verifyPayment).toHaveBeenCalledWith('pay-1', 'REJECT', 'Bukti buram'),
    )
  })

  it('menonaktifkan tombol kirim penolakan tanpa alasan', async () => {
    renderPage()

    await userEvent.click(await screen.findByRole('button', { name: 'Tolak' }))
    expect(await screen.findByRole('button', { name: 'Kirim penolakan' })).toBeDisabled()
  })

  it('menampilkan pesan saat daftar kosong', async () => {
    mocked.listPayments.mockResolvedValue({ data: [], meta: { page: 1, limit: 20, total: 0 } })
    renderPage()

    expect(await screen.findByText('Tidak ada pembayaran pada status ini.')).toBeInTheDocument()
  })
})
