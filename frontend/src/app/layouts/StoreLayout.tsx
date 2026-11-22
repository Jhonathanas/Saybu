import { Link, NavLink, Outlet, useNavigate } from 'react-router'
import { Button } from '@/components/ui/Button'
import { BottomNav } from '@/components/ui/BottomNav'
import { useAuth } from '@/features/auth/AuthProvider'
import { useCart } from '@/features/cart/store'

function SearchIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" className="size-5" aria-hidden="true">
      <circle cx="11" cy="11" r="6.5" />
      <path d="m16 16 4 4" strokeLinecap="round" />
    </svg>
  )
}

function CartIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" className="size-5" aria-hidden="true">
      <path d="M3 4h2l2.2 10.5h9.6L20 7H6" strokeLinecap="round" strokeLinejoin="round" />
      <circle cx="9" cy="19" r="1.4" />
      <circle cx="17" cy="19" r="1.4" />
    </svg>
  )
}

export function StoreLayout() {
  const { user, logout } = useAuth()
  const navigate = useNavigate()
  const cartCount = useCart((s) => s.lines.length)

  async function handleLogout() {
    await logout()
    navigate('/')
  }

  const cartLabel = cartCount > 0 ? `Keranjang, ${cartCount} jenis produk` : 'Keranjang'

  return (
    <div className="flex min-h-screen flex-col">
      <header className="sticky top-0 z-10 border-b border-border bg-card/95 backdrop-blur">
        <div className="mx-auto flex max-w-5xl items-center justify-between gap-3 px-4 py-3">
          <Link to="/" className="text-lg font-extrabold text-foreground">
            Saybu<span className="text-foreground">.</span>
          </Link>

          {/* Mobile: hanya logo + akses cepat; navigasi lengkap ada di BottomNav. */}
          <nav className="flex items-center gap-1 sm:hidden">
            <NavLink
              to="/produk"
              aria-label="Cari produk"
              className={({ isActive }) =>
                `tap-target inline-flex items-center rounded-lg px-2 ${
                  isActive ? 'text-foreground' : 'text-muted-foreground'
                }`
              }
            >
              <SearchIcon />
            </NavLink>
            <NavLink
              to="/keranjang"
              aria-label={cartLabel}
              className={({ isActive }) =>
                `tap-target relative inline-flex items-center rounded-lg px-2 ${
                  isActive ? 'text-foreground' : 'text-muted-foreground'
                }`
              }
            >
              <CartIcon />
              {cartCount > 0 && (
                <span className="absolute right-0 top-0 inline-flex min-w-4 items-center justify-center rounded-full bg-foreground px-1 text-[10px] font-semibold leading-4 text-background">
                  {cartCount}
                </span>
              )}
            </NavLink>
          </nav>

          <nav className="hidden items-center gap-1 text-sm sm:flex">
            <NavLink
              to="/"
              className={({ isActive }) =>
                `tap-target inline-flex items-center rounded-lg px-3 ${
                  isActive ? 'text-foreground' : 'text-muted-foreground hover:bg-muted'
                }`
              }
            >
              Beranda
            </NavLink>

            <NavLink
              to="/produk"
              className={({ isActive }) =>
                `tap-target inline-flex items-center rounded-lg px-3 ${
                  isActive ? 'text-foreground' : 'text-muted-foreground hover:bg-muted'
                }`
              }
            >
              Produk
            </NavLink>

            <NavLink
              to="/keranjang"
              aria-label={cartLabel}
              className={({ isActive }) =>
                `tap-target relative inline-flex items-center rounded-lg px-3 ${
                  isActive ? 'text-foreground' : 'text-muted-foreground hover:bg-muted'
                }`
              }
            >
              Keranjang
              {cartCount > 0 && (
                <span className="ml-1 inline-flex min-w-5 items-center justify-center rounded-full bg-foreground px-1.5 text-xs font-semibold text-background">
                  {cartCount}
                </span>
              )}
            </NavLink>

            <NavLink
              to="/pesanan"
              className={({ isActive }) =>
                `tap-target inline-flex items-center rounded-lg px-3 ${
                  isActive ? 'text-foreground' : 'text-muted-foreground hover:bg-muted'
                }`
              }
            >
              Lacak
            </NavLink>

            {user ? (
              <>
                <NavLink
                  to="/akun"
                  className={({ isActive }) =>
                    `tap-target inline-flex items-center rounded-lg px-3 ${
                      isActive ? 'text-foreground' : 'text-muted-foreground hover:bg-muted'
                    }`
                  }
                >
                  Akun
                </NavLink>
                {user.role === 'admin' && (
                  <NavLink
                    to="/admin"
                    className="tap-target inline-flex items-center rounded-lg px-3 text-muted-foreground hover:bg-muted"
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
                className="tap-target inline-flex items-center rounded-lg bg-primary px-3 text-sm font-medium text-background hover:bg-primary/90"
              >
                Masuk
              </Link>
            )}
          </nav>
        </div>
      </header>

      <main className="bottom-nav-space mx-auto w-full max-w-5xl flex-1 px-4 pt-4 sm:py-6">
        <Outlet />
      </main>

      <footer className="hidden border-t border-border bg-card sm:block">
        <div className="mx-auto max-w-5xl px-4 py-4 text-xs text-muted-foreground">
          © {new Date().getFullYear()} Saybu — buah &amp; sayur segar.
        </div>
      </footer>

      <BottomNav />
    </div>
  )
}
