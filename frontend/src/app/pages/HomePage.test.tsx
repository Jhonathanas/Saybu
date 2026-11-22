import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/features/auth/AuthProvider', () => ({
  useAuth: vi.fn(),
}))

vi.mock('@/features/catalog/api', () => ({
  listCategories: vi.fn(),
  listProducts: vi.fn(),
}))

import { useAuth } from '@/features/auth/AuthProvider'
import * as catalogApi from '@/features/catalog/api'
import type { Category, Product } from '@/features/catalog/types'
import { HomePage } from './HomePage'

const mockedAuth = vi.mocked(useAuth)
const mockedApi = vi.mocked(catalogApi)

const categories: Category[] = [
  { id: 'c-1', name: 'Buah', slug: 'buah', sort: 1 },
  { id: 'c-2', name: 'Sayur', slug: 'sayur', sort: 2 },
]

function product(id: string, name: string): Product {
  return {
    id,
    slug: id,
    name,
    description: '',
    unit: 'ikat',
    price: 5000,
    stock: 40,
    min_qty: 1,
    step_qty: 1,
    is_available: true,
    images: [],
    category: categories[1],
  }
}

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <HomePage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockedAuth.mockReturnValue({
    user: null,
    loading: false,
    login: vi.fn(),
    register: vi.fn(),
    logout: vi.fn(),
    refreshUser: vi.fn(),
  })
  mockedApi.listCategories.mockResolvedValue(categories)
  mockedApi.listProducts.mockResolvedValue({
    data: [product('bayam-segar', 'Bayam Segar')],
    meta: { page: 1, limit: 6, total: 1, total_pages: 1 },
  })
})

describe('HomePage', () => {
  it('menampilkan hero, kategori cepat, dan produk terbaru', async () => {
    renderPage()
    expect(screen.getByRole('heading', { level: 1, name: /Buah & sayur segar/ })).toBeInTheDocument()
    expect(await screen.findByText('Belanja cepat')).toBeInTheDocument()
    expect(await screen.findByRole('link', { name: /Buah/ })).toBeInTheDocument()
    expect(await screen.findByRole('link', { name: /Hemat/ })).toHaveAttribute(
      'href',
      '/produk?sort=price_asc',
    )
    expect(await screen.findByText('Bayam Segar')).toBeInTheDocument()
  })

  it('menampilkan ajakan daftar untuk pengunjung', () => {
    renderPage()
    expect(screen.getByRole('link', { name: 'Daftar akun' })).toBeInTheDocument()
  })
})
