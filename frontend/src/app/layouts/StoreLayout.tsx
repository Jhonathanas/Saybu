import { Link, NavLink, Outlet, useNavigate } from 'react-router'
import { Button } from '@/components/ui/Button'
import { useAuth } from '@/features/auth/AuthProvider'
import { useCart } from '@/features/cart/store'

export function StoreLayout() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const cartCount = useCart((s) => s.lines.length)

  async function handleLogout() {
    await logout()
    navigate('/')
  }

  return (
    <div className="flex min-h-screen flex-col">
      <header className="sticky top-0 z-10 border-b border-stone-200 bg-white/95 backdrop-blur">
        <div className="mx-auto flex max-w-5xl items-center justify-between gap-3 px-4 py-3">
          <Link to="/" className="text-lg font-bold text-brand-700">
            Saybu
          </Link>

          <nav className="flex items-center gap-1 text-sm">
            <NavLink
              to="/"
              className={({ isActive }) =>
                `tap-target inline-flex items-center rounded-lg px-3 ${
                  isActive ? 'text-brand-700' : 'text-stone-600 hover:bg-stone-100'
                }`
              }
            >
              Beranda
            </NavLink>

            <NavLink
              to="/produk"
              className={({ isActive }) =>
                `tap-target inline-flex items-center rounded-lg px-3 ${
                  isActive ? 'text-brand-700' : 'text-stone-600 hover:bg-stone-100'
                }`
              }
            >
              Produk
            </NavLink>

            <NavLink
              to="/keranjang"
              aria-label={
                cartCount > 0 ? `Keranjang, ${cartCount} jenis produk` : 'Keranjang'
              }
              className={({ isActive }) =>
                `tap-target relative inline-flex items-center rounded-lg px-3 ${
                  isActive ? 'text-brand-700' : 'text-stone-600 hover:bg-stone-100'
                }`
              }
            >
              Keranjang
              {cartCount > 0 && (
                <span className="ml-1 inline-flex min-w-5 items-center justify-center rounded-full bg-brand-600 px-1.5 text-xs font-semibold text-white">
                  {cartCount}
                </span>
              )}
            </NavLink>

            {user ? (
              <>
                <NavLink
                  to="/akun"
                  className={({ isActive }) =>
                    `tap-target inline-flex items-center rounded-lg px-3 ${
                      isActive ? 'text-brand-700' : 'text-stone-600 hover:bg-stone-100'
                    }`
                  }
                >
                  Akun
                </NavLink>
                {user.role === 'admin' && (
                  <NavLink
                    to="/admin"
                    className="tap-target inline-flex items-center rounded-lg px-3 text-stone-600 hover:bg-stone-100"
                  >
                    Admin
                  </NavLink>
                )}
                <Button variant="ghost" size="sm" onClick={handleLogout}>
                  Keluar
                </Button>
              </>
            ) : (
              <Link
                to="/masuk"
                className="tap-target inline-flex items-center rounded-lg bg-brand-600 px-3 text-sm font-medium text-white hover:bg-brand-700"
              >
                Masuk
              </Link>
            )}
          </nav>
        </div>
      </header>

      <main className="mx-auto w-full max-w-5xl flex-1 px-4 py-6">
        <Outlet />
      </main>

      <footer className="border-t border-stone-200 bg-white">
        <div className="mx-auto max-w-5xl px-4 py-4 text-xs text-stone-500">
          © {new Date().getFullYear()} Saybu — buah & sayur segar.
        </div>
      </footer>
    </div>
  )
}
