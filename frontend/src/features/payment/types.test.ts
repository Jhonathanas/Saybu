import { describe, expect, it } from 'vitest'
import {
  canUploadProof,
  paymentStatusBadgeClass,
  paymentStatusLabel,
} from './types'

describe('payment types', () => {
  it('menerjemahkan status pembayaran ke label Indonesia', () => {
    expect(paymentStatusLabel('UNPAID')).toBe('Belum dibayar')
    expect(paymentStatusLabel('WAITING_VERIFICATION')).toBe('Menunggu verifikasi')
    expect(paymentStatusLabel('PAID')).toBe('Sudah dibayar')
    expect(paymentStatusLabel('REJECTED')).toBe('Bukti ditolak')
    expect(paymentStatusLabel('COD_PENDING')).toBe('Bayar di tempat')
  })

  it('memakai status mentah bila tidak dikenal', () => {
    expect(paymentStatusLabel('NOPE')).toBe('NOPE')
  })

  it('memberi kelas badge token shadcn sesuai nada status', () => {
    expect(paymentStatusBadgeClass('PAID')).toContain('bg-foreground')
    expect(paymentStatusBadgeClass('REJECTED')).toContain('text-destructive')
    expect(paymentStatusBadgeClass('UNKNOWN')).toContain('bg-muted')
    expect(paymentStatusBadgeClass('CANCELLED')).toContain('text-muted-foreground')
  })

  it('mengizinkan unggah bukti hanya saat UNPAID atau REJECTED', () => {
    expect(canUploadProof('UNPAID')).toBe(true)
    expect(canUploadProof('REJECTED')).toBe(true)
    expect(canUploadProof('WAITING_VERIFICATION')).toBe(false)
    expect(canUploadProof('PAID')).toBe(false)
    expect(canUploadProof('COD_PENDING')).toBe(false)
  })
})
