import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it } from 'vitest'
import { useCart } from '@/features/cart/store'
import { ProductCard } from './ProductCard'
import type { Product } from './types'

const product: Product = {
  id: 'p-1',
  slug: 'bayam-segar',
  name: 'Bayam Segar',
  category: { id: 'c-1', name: 'Sayur', slug: 'sayur', sort: 2 },
  description: 'Bayam hijau.',
  unit: 'ikat',
  price: 5000,
  stock: 40,
  min_qty: 1,
  step_qty: 1,
  is_available: true,
  images: [],
}

function renderCard(p: Product = product) {
  return render(
    <MemoryRouter>
      <ProductCard product={p} />
    </MemoryRouter>,
  )
}

beforeEach(() => {
  useCart.getState().clear()
})

describe('ProductCard', () => {
  it('menampilkan nama, harga per satuan, dan stok', () => {
    renderCard()
    expect(screen.getByText('Bayam Segar')).toBeInTheDocument()
    expect(screen.getByText(/Rp\s?5\.000/)).toBeInTheDocument()
    expect(screen.getByText(/Stok 40 ikat/)).toBeInTheDocument()
  })

  it('menambahkan ke keranjang dengan qty minimum saat tombol diklik', async () => {
    renderCard({ ...product, unit: 'kg', price: 18000, stock: 10000, min_qty: 250, step_qty: 250 })
    await userEvent.click(screen.getByRole('button', { name: /Tambah Bayam Segar/ }))
    const lines = useCart.getState().lines
    expect(lines).toHaveLength(1)
    expect(lines[0].qty).toBe(250)
    expect(useCart.getState().subtotal()).toBe(4500)
  })

  it('menonaktifkan tombol saat stok habis', () => {
    renderCard({ ...product, is_available: false, stock: 0 })
    const button = screen.getByRole('button', { name: /Tambah Bayam Segar/ })
    expect(button).toBeDisabled()
    // Label "Stok habis" muncul di info stok dan di tombol.
    expect(screen.getAllByText('Stok habis').length).toBeGreaterThanOrEqual(1)
  })
})
