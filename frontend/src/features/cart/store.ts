import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { isWeightUnit, type Product, type Unit } from '@/features/catalog/types'

export interface CartLine {
  productId: string
  slug: string
  name: string
  unit: Unit
  /** Harga per satuan, rupiah penuh. */
  price: number
  /** Jumlah dalam satuan terkecil (gram untuk kg). */
  qty: number
  minQty: number
  stepQty: number
  /** Stok saat terakhir ditambahkan (untuk validasi di UI). */
  stock: number
  image?: string
}

export const CART_STORAGE_KEY = 'saybu-cart'

/** lineTotal menghitung total satu baris keranjang dalam rupiah penuh. */
export function lineTotal(line: Pick<CartLine, 'unit' | 'price' | 'qty'>): number {
  const raw = isWeightUnit(line.unit) ? (line.price * line.qty) / 1000 : line.price * line.qty
  return Math.round(raw)
}

/** clampQty membatasi qty agar sesuai stok, minimum, dan langkah satuan. */
export function clampQty(qty: number, minQty: number, stepQty: number, stock: number): number {
  const step = stepQty > 0 ? stepQty : 1
  const min = minQty > 0 ? minQty : step
  if (stock < min) return 0
  let value = Math.round(qty / step) * step
  if (value < min) value = min
  if (value > stock) value = Math.floor(stock / step) * step
  if (value < min) value = 0
  return value
}

interface CartState {
  lines: CartLine[]
  /** add menambahkan produk; qty digabung dengan baris yang ada. */
  add: (product: Product, qty: number) => void
  setQty: (productId: string, qty: number) => void
  remove: (productId: string) => void
  clear: () => void
  /** count mengembalikan jumlah baris (bukan total qty, karena satuan berbeda). */
  count: () => number
  subtotal: () => number
  /** find mengembalikan baris keranjang untuk sebuah produk. */
  find: (productId: string) => CartLine | undefined
}

function toLine(product: Product, qty: number): CartLine {
  return {
    productId: product.id,
    slug: product.slug,
    name: product.name,
    unit: product.unit,
    price: product.price,
    qty,
    minQty: product.min_qty,
    stepQty: product.step_qty,
    stock: product.stock,
    image: product.images[0],
  }
}

export const useCart = create<CartState>()(
  persist(
    (set, get) => ({
      lines: [],

      add(product, qty) {
        const lines = [...get().lines]
        const index = lines.findIndex((l) => l.productId === product.id)
        const step = product.step_qty > 0 ? product.step_qty : 1
        const min = product.min_qty > 0 ? product.min_qty : step

        if (index >= 0) {
          const merged = clampQty(lines[index].qty + qty, min, step, product.stock)
          if (merged === 0) {
            lines.splice(index, 1)
          } else {
            lines[index] = { ...toLine(product, merged) }
          }
        } else {
          const clamped = clampQty(qty, min, step, product.stock)
          if (clamped > 0) lines.push(toLine(product, clamped))
        }
        set({ lines })
      },

      setQty(productId, qty) {
        const lines = [...get().lines]
        const index = lines.findIndex((l) => l.productId === productId)
        if (index < 0) return
        const line = lines[index]
        // Qty 0 atau negatif berarti pengguna ingin menghapus baris.
        if (qty <= 0) {
          lines.splice(index, 1)
          set({ lines })
          return
        }
        const clamped = clampQty(qty, line.minQty, line.stepQty, line.stock)
        if (clamped === 0) {
          lines.splice(index, 1)
        } else {
          lines[index] = { ...line, qty: clamped }
        }
        set({ lines })
      },

      remove(productId) {
        set({ lines: get().lines.filter((l) => l.productId !== productId) })
      },

      clear() {
        set({ lines: [] })
      },

      count() {
        return get().lines.length
      },

      subtotal() {
        return get().lines.reduce((sum, line) => sum + lineTotal(line), 0)
      },

      find(productId) {
        return get().lines.find((l) => l.productId === productId)
      },
    }),
    {
      name: CART_STORAGE_KEY,
      version: 1,
    },
  ),
)
