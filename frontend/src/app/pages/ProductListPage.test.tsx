import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'

vi.mock('@/features/catalog/api', () => ({
  listCategories: vi.fn(),
  listProducts: vi.fn(),
}))

import * as catalogApi from '@/features/catalog/api'
import type { Category, Product } from '@/features/catalog/types'
import { ProductListPage } from './ProductListPage'

const mockedApi = vi.mocked(catalogApi)

const categories: Category[] = [
  { id: 'c-1', name: 'Buah', slug: 'buah', sort: 1 },
  { id: 'c-2', name: 'Sayur', slug: 'sayur', sort: 2 },
]

const products: Product[] = [
  {
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
    category: categories[1],
  },
]

function renderPage() {
  const queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={queryClient}>
      <MemoryRouter>
        <ProductListPage />
      </MemoryRouter>
    </QueryClientProvider>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  mockedApi.listCategories.mockResolvedValue(categories)
  mockedApi.listProducts.mockResolvedValue({
    data: products,
    meta: { page: 1, limit: 12, total: 1, total_pages: 1 },
  })
})

describe('ProductListPage', () => {
  it('menampilkan produk dalam grid dua kolom di mobile', async () => {
    renderPage()
    expect(screen.getByRole('heading', { name: 'Produk' })).toBeInTheDocument()
    expect(await screen.findByText('Bayam Segar')).toBeInTheDocument()
    expect(screen.getByRole('button', { name: 'Tambah Bayam Segar ke keranjang' })).toBeInTheDocument()
  })

  it('mencari produk lewat form', async () => {
    const user = userEvent.setup()
    renderPage()
    await user.type(await screen.findByLabelText('Cari produk'), 'bayam')
    await user.click(screen.getByRole('button', { name: 'Cari' }))
    expect(mockedApi.listProducts).toHaveBeenLastCalledWith(
      expect.objectContaining({ q: 'bayam' }),
    )
  })

  it('memfilter produk berdasarkan kategori', async () => {
    const user = userEvent.setup()
    renderPage()
    await user.click(await screen.findByRole('button', { name: 'Sayur' }))
    expect(mockedApi.listProducts).toHaveBeenLastCalledWith(
      expect.objectContaining({ category: 'sayur' }),
    )
  })
})
