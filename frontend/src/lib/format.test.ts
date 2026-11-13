import { describe, expect, it } from 'vitest'
import { formatDate, formatDateTime, formatIDR, formatPricePerUnit, formatQty } from './format'

describe('formatIDR', () => {
  it('memformat rupiah tanpa desimal', () => {
    // Intl memakai non-breaking space; normalisasi dulu.
    const normalized = formatIDR(15000).replace(/\u00a0/g, ' ')
    expect(normalized).toBe('Rp 15.000')
  })

  it('menangani nol', () => {
    expect(formatIDR(0).replace(/\u00a0/g, ' ')).toBe('Rp 0')
  })

  it('memberi tanda hubung untuk nilai tak valid', () => {
    expect(formatIDR(Number.NaN)).toBe('—')
  })
})

describe('formatDateTime', () => {
  it('memformat waktu dalam zona Asia/Jakarta', () => {
    const result = formatDateTime('2026-01-02T03:00:00Z')
    // 03:00 UTC = 10:00 WIB.
    expect(result).toContain('10.00')
    expect(result).toContain('2026')
  })

  it('memberi tanda hubung untuk tanggal tak valid', () => {
    expect(formatDateTime('bukan-tanggal')).toBe('—')
  })
})

describe('formatDate', () => {
  it('memformat tanggal panjang', () => {
    expect(formatDate('2026-01-02T03:00:00Z')).toContain('2026')
  })
})

describe('formatQty', () => {
  it('menampilkan kg dari nilai gram', () => {
    expect(formatQty('kg', 1500)).toBe('1,5 kg')
  })

  it('menampilkan satuan non-kg apa adanya', () => {
    expect(formatQty('ikat', 3)).toBe('3 ikat')
  })

  it('memberi tanda hubung untuk nilai tak valid', () => {
    expect(formatQty('kg', Number.NaN)).toBe('—')
  })
})

describe('formatPricePerUnit', () => {
  it('menyertakan satuan', () => {
    const label = formatPricePerUnit(18000, 'kg').replace(/\u00a0/g, ' ')
    expect(label).toBe('Rp 18.000 / kg')
  })
})
