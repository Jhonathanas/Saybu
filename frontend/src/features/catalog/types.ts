export type Unit = 'kg' | 'ikat' | 'pcs' | 'buah' | 'paket'

export const UNITS: Unit[] = ['kg', 'ikat', 'pcs', 'buah', 'paket']

/** Satuan yang stok/qty-nya disimpan dalam gram. */
export function isWeightUnit(unit: string): boolean {
  return unit === 'kg'
}

export interface Category {
  id: string
  name: string
  slug: string
  sort: number
}

export interface Product {
  id: string
  slug: string
  name: string
  category?: Category
  description: string
  unit: Unit
  /** Harga per satuan, rupiah penuh. */
  price: number
  /** Stok dalam satuan terkecil (gram untuk kg). */
  stock: number
  min_qty: number
  step_qty: number
  is_available: boolean
  images: string[]
  created_at?: string
  updated_at?: string
}

export interface AdminProduct {
  id: string
  slug: string
  name: string
  category?: Category
  category_id: string
  description: string
  unit: Unit
  price: number
  /** Stok dalam satuan terkecil (gram untuk kg). */
  stock_qty: number
  min_qty: number
  step_qty: number
  is_active: boolean
  images: string[]
  created_at?: string
  updated_at?: string
}

export interface ListMeta {
  page: number
  limit: number
  total: number
  total_pages: number
}

/** Paged adalah respons berhalaman: daftar data + meta paginasi. */
export interface Paged<T> {
  data: T[]
  meta?: ListMeta
}

export type ProductSort = 'newest' | 'price_asc' | 'price_desc'

export interface ProductQuery {
  category?: string
  q?: string
  sort?: ProductSort
  page?: number
  limit?: number
}

export interface ProductInput {
  name: string
  slug?: string
  category_id?: string
  description?: string
  unit: Unit
  price: number
  stock_qty: number
  min_qty?: number
  step_qty?: number
  is_active: boolean
  images?: string[]
}

export interface ProductPatch {
  name?: string
  slug?: string
  category_id?: string
  description?: string
  unit?: Unit
  price?: number
  min_qty?: number
  step_qty?: number
  is_active?: boolean
  images?: string[]
}

export type StockMode = 'set' | 'adjust'
