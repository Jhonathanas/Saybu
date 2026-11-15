import { describe, expect, it } from 'vitest'
import { isCancellable, isUnpaid, paymentLabel, statusLabel } from './status'

describe('status', () => {
  it('menerjemahkan status ke label Indonesia', () => {
    expect(statusLabel('PENDING_PAYMENT')).toBe('Menunggu Pembayaran')
    expect(statusLabel('COMPLETED')).toBe('Selesai')
    expect(statusLabel('EXPIRED')).toBe('Kedaluwarsa')
  })

  it('memakai status mentah bila tidak dikenal', () => {
    expect(statusLabel('UNKNOWN')).toBe('UNKNOWN')
  })

  it('menandai pesanan yang masih bisa dibatalkan', () => {
    expect(isCancellable('PENDING_PAYMENT')).toBe(true)
    expect(isCancellable('PAID')).toBe(true)
    expect(isCancellable('SHIPPED')).toBe(false)
    expect(isCancellable('COMPLETED')).toBe(false)
  })

  it('menandai pesanan yang belum dibayar', () => {
    expect(isUnpaid('PENDING_PAYMENT')).toBe(true)
    expect(isUnpaid('WAITING_VERIFICATION')).toBe(true)
    expect(isUnpaid('PAID')).toBe(false)
  })

  it('memberi label metode pembayaran', () => {
    expect(paymentLabel('COD')).toMatch(/COD/)
    expect(paymentLabel('TRANSFER')).toMatch(/Transfer/)
  })
})
