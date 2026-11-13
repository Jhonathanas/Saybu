import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it } from 'vitest'
import { useCart } from '@/features/cart/store'
import { CartPage } from './CartPage'
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

function renderPage() {
  return render(
    <MemoryRouter>
      <CartPage />
    </MemoryRouter>,
  )
}

beforeEach(() => {
  useCart.getState().clear()
})

describe('CartPage', () => {
  it('menampilkan ajakan belanja saat keranjang kosong', () => {
    renderPage()
    expect(screen.getByText(/Keranjang masih kosong/)).toBeInTheDocument()
  })

  it('menampilkan baris dan subtotal', () => {
    useCart.getState().add(product(), 2)
    renderPage()
    expect(screen.getByText('Bayam Segar')).toBeInTheDocument()
    // Baris 2 x Rp5.000 dan subtotal.
    expect(screen.getAllByText(/Rp\s?10\.000/).length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText('Subtotal')).toBeInTheDocument()
  })

  it('menghapus baris', async () => {
    useCart.getState().add(product(), 1)
    renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Hapus' }))
    expect(useCart.getState().lines).toHaveLength(0)
    expect(screen.getByText(/Keranjang masih kosong/)).toBeInTheDocument()
  })

  it('mengosongkan keranjang', async () => {
    useCart.getState().add(product(), 1)
    renderPage()
    await userEvent.click(screen.getByRole('button', { name: 'Kosongkan' }))
    expect(useCart.getState().lines).toHaveLength(0)
  })
})
