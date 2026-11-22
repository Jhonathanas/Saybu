import { NavLink } from 'react-router'
import { House, LayoutGrid, MapPin, ShoppingCart, UserRound } from 'lucide-react'
import { Badge } from './shadcn/badge'
import { useAuth } from '@/features/auth/AuthProvider'
import { useCart } from '@/features/cart/store'
import { cn } from '@/lib/utils'

/** Navigasi bawah untuk layar mobile. Disembunyikan mulai breakpoint sm. */
export function BottomNav() {
  const { user } = useAuth()
  const cartCount = useCart((s) => s.lines.length)

  const items = [
    { to: '/', label: 'Beranda', end: true, Icon: House },
    { to: '/produk', label: 'Produk', end: false, Icon: LayoutGrid },
    { to: '/keranjang', label: 'Keranjang', end: false, Icon: ShoppingCart, badge: cartCount },
    { to: '/pesanan', label: 'Lacak', end: false, Icon: MapPin },
    {
      to: user ? '/akun' : '/masuk',
      label: user ? 'Akun' : 'Masuk',
      end: false,
      Icon: UserRound,
    },
  ]

  return (
    <nav
      aria-label="Navigasi bawah"
      className="fixed inset-x-0 bottom-0 z-20 border-t border-border bg-background/95 pb-[env(safe-area-inset-bottom)] backdrop-blur sm:hidden"
    >
      <ul className="mx-auto grid max-w-md grid-cols-5">
        {items.map(({ to, label, end, Icon, badge }) => (
          <li key={label}>
            <NavLink
              to={to}
              end={end}
              aria-label={badge ? `${label}, ${badge} jenis produk` : label}
              className={({ isActive }) =>
                cn(
                  'tap-target relative flex h-16 flex-col items-center justify-center gap-0.5 text-[11px] font-medium',
                  isActive ? 'text-foreground' : 'text-muted-foreground',
                )
              }
            >
              {({ isActive }) => (
                <>
                  <span
                    className={cn(
                      'absolute inset-x-3 top-0 h-0.5 rounded-b-full',
                      isActive ? 'bg-foreground' : 'bg-transparent',
                    )}
                  />
                  <span className="relative">
                    <Icon aria-hidden="true" className="size-5" />
                    {badge ? (
                      <Badge className="absolute -right-2.5 -top-1.5 min-w-4 px-1 text-[10px] leading-4">
                        {badge}
                      </Badge>
                    ) : null}
                  </span>
                  <span>{label}</span>
                </>
              )}
            </NavLink>
          </li>
        ))}
      </ul>
    </nav>
  )
}
