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

describe('OrderDetailView', () => {
  it('menampilkan kode, item, dan total', () => {
    render(<OrderDetailView order={order()} />)
    expect(screen.getByText('SB-20261114-0001')).toBeInTheDocument()
    expect(screen.getByText('Semangka Merah')).toBeInTheDocument()
    expect(screen.getAllByText('Menunggu Pembayaran').length).toBeGreaterThanOrEqual(1)
    expect(screen.getAllByText(/Rp\s?98\.000/).length).toBeGreaterThanOrEqual(1)
  })

  it('menampilkan instruksi transfer bank saat belum dibayar', () => {
    render(<OrderDetailView order={order()} showPaymentInstructions />)
    expect(screen.getByText('Instruksi pembayaran')).toBeInTheDocument()
    expect(screen.getByText('1234567890')).toBeInTheDocument()
    expect(screen.getByText('Saybu Buah & Sayur')).toBeInTheDocument()
  })

  it('tidak menampilkan instruksi transfer untuk COD', () => {
    render(<OrderDetailView order={order({ payment_method: 'COD' })} showPaymentInstructions />)
    expect(screen.queryByText('Instruksi pembayaran')).not.toBeInTheDocument()
    expect(screen.getByText(/bayar saat pesanan diterima/i)).toBeInTheDocument()
  })

  it('menyembunyikan instruksi transfer saat status bukan menunggu bayar', () => {
    render(<OrderDetailView order={order({ status: 'PAID' })} showPaymentInstructions />)
    expect(screen.queryByText('Instruksi pembayaran')).not.toBeInTheDocument()
  })
})
