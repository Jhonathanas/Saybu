import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { OrderDetailView } from './OrderDetailView'
import type { Order } from './types'

function order(overrides: Partial<Order> = {}): Order {
  return {
    id: 'o-1',
    code: 'SB-20261114-0001',
    status: 'PENDING_PAYMENT',
    fulfillment: 'DELIVERY',
    payment_method: 'TRANSFER',
    customer: { name: 'Siti', phone: '081234567890', note: 'Pilih yang segar' },
    address: { line: 'Jl. Melati 12', city: 'Bandung' },
    zone_id: 'zone-1',
    zone_name: 'Kec. Sukamaju',
    subtotal: 88000,
    shipping_fee: 10000,
    total: 98000,
    items: [
      {
        product_id: 'p-1',
        name: 'Semangka Merah',
        unit: 'buah',
        qty: 1,
        qty_display: '1 buah',
        unit_price: 35000,
        line_total: 35000,
      },
    ],
    status_history: [{ status: 'PENDING_PAYMENT', at: '2026-11-14T03:00:00Z' }],
    expires_at: '2026-11-15T03:00:00Z',
    created_at: '2026-11-14T03:00:00Z',
    ...overrides,
  }
}

// Instruksi pembayaran & unggah bukti kini berada di PaymentPanel
// (features/payment), jadi tidak diuji di sini.
describe('OrderDetailView', () => {
  it('menampilkan kode, item, dan total', () => {
    render(<OrderDetailView order={order()} />)
    expect(screen.getByText('SB-20261114-0001')).toBeInTheDocument()
    expect(screen.getByText('Semangka Merah')).toBeInTheDocument()
    expect(screen.getAllByText('Menunggu Pembayaran').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText(/Rp\s?98\.000/).length).toBeGreaterThanOrEqual(1)
  })

  it('menampilkan alasan batal bila ada', () => {
    render(<OrderDetailView order={order({ status: 'CANCELLED', cancel_reason: 'Stok habis' })} />)
    expect(screen.getByText('Alasan batal: Stok habis')).toBeInTheDocument()
  })

  it('menampilkan tombol batalkan pesanan bila diberi handler', () => {
    render(<OrderDetailView order={order()} onCancel={() => {}} />)
    expect(screen.getByRole('button', { name: 'Batalkan pesanan' })).toBeInTheDocument()
  })
})
