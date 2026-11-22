import { render, screen } from '@testing-library/react'
import { MemoryRouter } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { User } from '@/features/auth/types'
import type { Product } from '@/features/catalog/types'

vi.mock('@/features/auth/AuthProvider', () => ({
  useAuth: vi.fn(),
}))

import { useAuth } from '@/features/auth/AuthProvider'
import { useCart } from '@/features/cart/store'
import { BottomNav } from './BottomNav'

const mockedAuth = vi.mocked(useAuth)

const USER: User = {
  id: 'u-1',
  name: 'Sari',
  phone: '081234567890',
  email: 'sari@example.com',
  role: 'customer',
}

const product: Product = {
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
}

function renderNav() {
  return render(
    <MemoryRouter>
      <BottomNav />
    </MemoryRouter>,
  )
}

beforeEach(() => {
  vi.clearAllMocks()
  useCart.getState().clear()
  mockedAuth.mockReturnValue({
    user: null,
    loading: false,
    login: vi.fn(),
    register: vi.fn(),
    logout: vi.fn(),
    refreshUser: vi.fn(),
  })
})

describe('BottomNav', () => {
  it('menampilkan lima tab untuk pengunjung', () => {
    renderNav()
    const nav = screen.getByRole('navigation', { name: 'Navigasi bawah' })
    expect(nav).toBeInTheDocument()
    expect(screen.getAllByRole('link')).toHaveLength(5)
    expect(screen.getByRole('link', { name: 'Masuk' })).toHaveAttribute('href', '/masuk')
  })

  it('menampilkan tab Akun untuk pengguna yang sudah masuk', () => {
    mockedAuth.mockReturnValue({
      user: USER,
      loading: false,
      login: vi.fn(),
      register: vi.fn(),
      logout: vi.fn(),
      refreshUser: vi.fn(),
    })
    renderNav()
    expect(screen.getByRole('link', { name: 'Akun' })).toHaveAttribute('href', '/akun')
  })

  it('menampilkan jumlah produk di badge keranjang', () => {
    useCart.getState().add(product, 2)
    renderNav()
    expect(
      screen.getByRole('link', { name: 'Keranjang, 1 jenis produk' }),
    ).toBeInTheDocument()
  })
})
