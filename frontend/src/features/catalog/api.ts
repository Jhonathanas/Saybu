import { request, requestWithMeta, uploadFile } from '@/lib/api'
import type {
  AdminProduct,
  Category,
  ListMeta,
  Paged,
  Product,
  ProductInput,
  ProductPatch,
  ProductQuery,
  StockMode,
} from './types'

function buildQuery(params: ProductQuery): string {
  const query = new URLSearchParams()
  if (params.category) query.set('category', params.category)
  if (params.q) query.set('q', params.q)
  if (params.sort) query.set('sort', params.sort)
  if (params.page) query.set('page', String(params.page))
  if (params.limit) query.set('limit', String(params.limit))
  const s = query.toString()
  return s ? `?${s}` : ''
}

export function listCategories(): Promise<Category[]> {
  return request<Category[]>('/categories', { auth: false })
}

export function listProducts(params: ProductQuery = {}): Promise<Paged<Product>> {
  return requestWithMeta<Product[], ListMeta>(`/products${buildQuery(params)}`, { auth: false })
}

export function getProduct(slug: string): Promise<Product> {
  return request<Product>(`/products/${encodeURIComponent(slug)}`, { auth: false })
}

// --- admin ---

export function adminListProducts(params: ProductQuery = {}): Promise<Paged<AdminProduct>> {
  return requestWithMeta<AdminProduct[], ListMeta>(`/admin/products${buildQuery(params)}`)
}

export function createProduct(input: ProductInput): Promise<AdminProduct> {
  return request<AdminProduct>('/admin/products', { method: 'POST', body: input })
}

export function updateProduct(id: string, patch: ProductPatch): Promise<AdminProduct> {
  return request<AdminProduct>(`/admin/products/${id}`, { method: 'PATCH', body: patch })
}

export function setStock(id: string, mode: StockMode, qty: number): Promise<AdminProduct> {
  return request<AdminProduct>(`/admin/products/${id}/stock`, {
    method: 'PATCH',
    body: { mode, qty },
  })
}

export function deleteProduct(id: string): Promise<void> {
  return request<void>(`/admin/products/${id}`, { method: 'DELETE' })
}

export function uploadImage(file: File): Promise<{ url: string }> {
  return uploadFile<{ url: string }>('/admin/uploads', file)
}
