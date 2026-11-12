import { render, screen } from '@testing-library/react'
import { MemoryRouter, Route, Routes } from 'react-router'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { User } from './types'

vi.mock('./AuthProvider', () => ({
  useAuth: vi.fn(),
}))

import { useAuth } from './AuthProvider'
import { RequireAuth, RequireRole } from './guards'

const mockedUseAuth = vi.mocked(useAuth)

function setAuth(user: User | null, loading = false) {
  mockedUseAuth.mockReturnValue({
    user,
    loading,
    login: vi.fn(),
    register: vi.fn(),
    logout: vi.fn(),
    refreshUser: vi.fn(),
  })
}

const admin: User = { id: '1', name: 'Admin', phone: '0812', email: '', role: 'admin' }
const customer: User = { id: '2', name: 'Sari', phone: '0813', email: '', role: 'customer' }

function renderRoutes() {
  return render(
    <MemoryRouter initialEntries={['/admin']}>
      <Routes>
        <Route path="/" element={<div>beranda</div>} />
        <Route path="/masuk" element={<div>halaman masuk</div>} />
        <Route path="/admin" element={<RequireRole role="admin" />}>
          <Route index element={<div>panel admin</div>} />
        </Route>
      </Routes>
    </MemoryRouter>,
  )
}

describe('RequireRole', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('mengizinkan admin', () => {
    setAuth(admin)
    renderRoutes()
    expect(screen.getByText('panel admin')).toBeInTheDocument()
  })

  it('menolak customer dan mengalihkan ke beranda', () => {
    setAuth(customer)
    renderRoutes()
    expect(screen.getByText('beranda')).toBeInTheDocument()
  })

  it('mengalihkan tamu ke halaman masuk', () => {
    setAuth(null)
    renderRoutes()
    expect(screen.getByText('halaman masuk')).toBeInTheDocument()
  })
})

describe('RequireAuth', () => {
  beforeEach(() => {
    vi.clearAllMocks()
  })

  it('menampilkan loader selama pemulihan sesi', () => {
    setAuth(null, true)
    render(
      <MemoryRouter initialEntries={['/akun']}>
        <Routes>
          <Route element={<RequireAuth />}>
            <Route path="/akun" element={<div>profil</div>} />
          </Route>
        </Routes>
      </MemoryRouter>,
    )
    expect(screen.getByRole('status')).toBeInTheDocument()
  })
})
