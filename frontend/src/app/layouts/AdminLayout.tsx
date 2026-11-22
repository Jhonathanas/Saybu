import { Link, NavLink, Outlet, useNavigate } from 'react-router'
import { Button } from '@/components/ui/Button'
import { useAuth } from '@/features/auth/AuthProvider'

const navItems = [
  { to: '/admin', label: 'Ringkasan', end: true },
  { to: '/admin/pesanan', label: 'Pesanan', end: false },
  { to: '/admin/produk', label: 'Produk', end: false },
  { to: '/admin/pembayaran', label: 'Pembayaran', end: false },
  { to: '/admin/pengguna', label: 'Pengguna', end: false },
]

export function AdminLayout() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()

  async function handleLogout() {
    await logout()
    navigate('/')
  }

  return (
    <div className="flex min-h-screen flex-col bg-muted">
      <header className="border-b border-border bg-card">
        <div className="mx-auto flex max-w-6xl items-center justify-between gap-3 px-4 py-3">
          <div className="flex items-baseline gap-3">
            <Link to="/admin" className="text-lg font-bold text-foreground">
              Saybu Admin
            </Link>
            <span className="hidden text-xs text-muted-foreground sm:inline">
              {user?.name}
            </span>
          </div>
          <div className="flex items-center gap-2">
            <Link to="/" className="text-sm text-muted-foreground hover:underline">
              Lihat toko
            </Link>
            <Button variant="secondary" size="sm" onClick={handleLogout}>
              Keluar
            </Button>
          </div>
        </div>
      </header>

      <div className="mx-auto flex w-full max-w-6xl flex-1 flex-col gap-6 px-4 py-6 md:flex-row">
        <nav className="flex gap-1 overflow-x-auto md:w-48 md:flex-col">
          {navItems.map((item) => (
            <NavLink
              key={item.to}
              to={item.to}
              end={item.end}
              className={({ isActive }) =>
                `tap-target inline-flex items-center whitespace-nowrap rounded-lg px-3 text-sm ${
                  isActive
                    ? 'bg-primary text-background'
                    : 'text-foreground hover:bg-accent'
                }`
              }
            >
              {item.label}
            </NavLink>
          ))}
        </nav>

        <main className="flex-1">
          <Outlet />
        </main>
      </div>
    </div>
  )
}
