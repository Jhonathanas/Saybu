import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { Order } from '@/features/checkout/types'
import { ApiError } from '@/lib/api'
import { PaymentPanel } from './PaymentPanel'
import * as api from './api'

vi.mock('./api', () => ({
  getPayment: vi.fn(),
  uploadPaymentProof: vi.fn(),
}))

vi.mock('./ProofUploadForm', () => ({
  ProofUploadForm: () => <div>Form unggah bukti</div>,
}))

const mocked = vi.mocked(api)

function order(overrides: Partial<Order> = {}): Order {
  return {
    id: 'o-1',
    code: 'SB-20261114-0001',
    status: 'PENDING_PAYMENT',
    fulfillment: 'DELIVERY',
    payment_method: 'TRANSFER',
    customer: { name: 'Siti', phone: '081234567890' },
    address: { line: 'Jl. Melati 12', city: 'Bandung' },
    zone_id: 'zone-1',
    zone_name: 'Kec. Sukamaju',
    subtotal: 88000,
    shipping_fee: 10000,
    total: 98000,
    items: [],
    status_history: [],
    expires_at: '2026-11-15T03:00:00Z',
    created_at: '2026-11-14T03:00:00Z',
    ...overrides,
  }
}

function renderPanel(o: Order = order()) {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <PaymentPanel order={o} phone="081234567890" />
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('PaymentPanel', () => {
  it('menampilkan instruksi transfer & form unggah saat UNPAID', async () => {
    mocked.getPayment.mockResolvedValue({
      method: 'TRANSFER',
      status: 'UNPAID',
      amount: 98000,
      bank: { name: 'BCA', account_number: '1234567890', account_holder: 'Saybu' },
    })
    renderPanel()

    expect(await screen.findByText('Belum dibayar')).toBeInTheDocument()
    expect(screen.getByText('1234567890')).toBeInTheDocument()
    expect(screen.getByText('Saybu')).toBeInTheDocument()
    expect(screen.getByText('Form unggah bukti')).toBeInTheDocument()
  })

  it('menyembunyikan form unggah saat menunggu verifikasi', async () => {
    mocked.getPayment.mockResolvedValue({
      method: 'TRANSFER',
      status: 'WAITING_VERIFICATION',
      amount: 98000,
    })
    renderPanel()

    expect(await screen.findByText('Menunggu verifikasi')).toBeInTheDocument()
    expect(screen.getByText(/sedang diverifikasi admin/i)).toBeInTheDocument()
    expect(screen.queryByText('Form unggah bukti')).not.toBeInTheDocument()
  })

  it('menampilkan pesan sukses saat sudah dibayar', async () => {
    mocked.getPayment.mockResolvedValue({
      method: 'TRANSFER',
      status: 'PAID',
      amount: 98000,
      verified_at: '2026-11-14T05:00:00Z',
    })
    renderPanel()

    expect(await screen.findByText(/Pembayaran sudah diverifikasi/)).toBeInTheDocument()
    expect(screen.queryByText('Form unggah bukti')).not.toBeInTheDocument()
  })

  it('menampilkan catatan COD tanpa form unggah', async () => {
    mocked.getPayment.mockResolvedValue({
      method: 'COD',
      status: 'COD_PENDING',
      amount: 98000,
      bank: null,
    })
    renderPanel(order({ payment_method: 'COD' }))

    expect(await screen.findByText(/bayar saat pesanan diterima/i)).toBeInTheDocument()
    expect(screen.queryByText('Form unggah bukti')).not.toBeInTheDocument()
  })

  it('menampilkan alasan saat bukti ditolak dan form unggah ulang', async () => {
    mocked.getPayment.mockResolvedValue({
      method: 'TRANSFER',
      status: 'REJECTED',
      amount: 98000,
      note: 'Bukti buram',
    })
    renderPanel()

    expect(await screen.findByText(/Bukti transfer ditolak: Bukti buram/)).toBeInTheDocument()
    expect(screen.getByText('Form unggah bukti')).toBeInTheDocument()
  })

  it('tidak menampilkan apa pun bila status pembayaran gagal dimuat', async () => {
    mocked.getPayment.mockRejectedValue(new ApiError(404, 'NOT_FOUND', 'Tidak ditemukan'))
    const { container } = renderPanel()

    expect(container).toBeEmptyDOMElement()
  })
})
