import { beforeEach, describe, expect, it } from 'vitest'
import { clampQty, lineTotal, useCart, type CartLine } from './store'
import type { Product } from '@/features/catalog/types'

function product(overrides: Partial<Product> = {}): Product {
  return {
    id: 'p-1',
    slug: 'bayam-segar',
    name: 'Bayam Segar',
    description: '',
    unit: 'ikat',
    price: 5000,
    stock: 40,
    min_qty: 1,
    step_qty: 1,
    is_available: true,
    images: [],
    ...overrides,
  }
}

beforeEach(() => {
  useCart.getState().clear()
})

describe('lineTotal', () => {
  it('menghitung harga satuan non-kg secara langsung', () => {
    const line: Pick<CartLine, 'unit' | 'price' | 'qty'> = { unit: 'ikat', price: 5000, qty: 3 }
    expect(lineTotal(line)).toBe(15000)
  })

  it('menghitung harga per kg dari qty gram', () => {
    const line: Pick<CartLine, 'unit' | 'price' | 'qty'> = { unit: 'kg', price: 18000, qty: 500 }
    expect(lineTotal(line)).toBe(9000)
  })

  it('membulatkan hasil untuk gram tidak bulat', () => {
    const line: Pick<CartLine, 'unit' | 'price' | 'qty'> = { unit: 'kg', price: 15500, qty: 333 }
    expect(lineTotal(line)).toBe(Math.round((15500 * 333) / 1000))
  })
})

describe('clampQty', () => {
  it('membulatkan ke langkah satuan', () => {
    expect(clampQty(320, 250, 250, 10000)).toBe(250)
    expect(clampQty(400, 250, 250, 10000)).toBe(500)
  })

  it('menaikkan ke minimum', () => {
    expect(clampQty(100, 250, 250, 10000)).toBe(250)
  })

  it('membatasi ke stok', () => {
    expect(clampQty(99999, 250, 250, 1000)).toBe(1000)
  })

  it('mengembalikan 0 bila stok di bawah minimum', () => {
    expect(clampQty(500, 250, 250, 100)).toBe(0)
  })
})

describe('useCart', () => {
  it('menambahkan produk baru dengan qty minimum', () => {
    useCart.getState().add(product({ min_qty: 1, step_qty: 1 }), 1)
    const lines = useCart.getState().lines
    expect(lines).toHaveLength(1)
    expect(lines[0].qty).toBe(1)
    expect(useCart.getState().count()).toBe(1)
    expect(useCart.getState().subtotal()).toBe(5000)
  })

  it('menggabungkan qty untuk produk yang sama', () => {
    useCart.getState().add(product(), 2)
    useCart.getState().add(product(), 3)
    expect(useCart.getState().lines).toHaveLength(1)
    expect(useCart.getState().lines[0].qty).toBe(5)
  })

  it('membatasi qty gabungan pada stok', () => {
    useCart.getState().add(product({ stock: 4 }), 3)
    useCart.getState().add(product({ stock: 4 }), 3)
    expect(useCart.getState().lines[0].qty).toBe(4)
  })

  it('menghapus baris ketika setQty menjadi 0', () => {
    useCart.getState().add(product(), 2)
    useCart.getState().setQty('p-1', 0)
    expect(useCart.getState().lines).toHaveLength(0)
  })

  it('menyimpan qty dalam gram untuk satuan kg', () => {
    useCart.getState().add(product({ unit: 'kg', price: 18000, stock: 10000, min_qty: 250, step_qty: 250 }), 250)
    expect(useCart.getState().lines[0].qty).toBe(250)
    expect(useCart.getState().subtotal()).toBe(4500)
  })

  it('menghitung subtotal gabungan beberapa baris', () => {
    useCart.getState().add(product({ id: 'p-1', price: 5000 }), 2)
    useCart.getState().add(
      product({ id: 'p-2', slug: 'tomat', name: 'Tomat', unit: 'kg', price: 14000, stock: 20000, min_qty: 250, step_qty: 250 }),
      1000,
    )
    expect(useCart.getState().subtotal()).toBe(5000 * 2 + 14000)
  })

  it('find mengembalikan baris produk', () => {
    useCart.getState().add(product(), 1)
    expect(useCart.getState().find('p-1')?.slug).toBe('bayam-segar')
    expect(useCart.getState().find('tidak-ada')).toBeUndefined()
  })

  it('remove menghapus baris', () => {
    useCart.getState().add(product(), 1)
    useCart.getState().remove('p-1')
    expect(useCart.getState().lines).toHaveLength(0)
  })
})
