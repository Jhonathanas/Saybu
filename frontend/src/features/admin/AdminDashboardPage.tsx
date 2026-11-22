import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
import { Card } from '@/components/ui/Card'
import { Alert, Spinner } from '@/components/ui/Feedback'
import { formatIDR, formatQty } from '@/lib/format'
import { getTodayStats } from '@/features/admin/api'
import { adminListProducts } from '@/features/catalog/api'
import { usePageTitle } from '@/lib/seo'

export function AdminDashboardPage() {
  usePageTitle('Ringkasan · Admin')
  const stats = useQuery({
    queryKey: ['admin', 'stats', 'today'],
    queryFn: getTodayStats,
  })

  const activeProducts = useQuery({
    queryKey: ['admin', 'products', { page: 1, limit: 1 }],
    queryFn: () => adminListProducts({ page: 1, limit: 1 }),
  })

  const activeCount = activeProducts.data?.meta?.total
  const s = stats.data

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-xl font-bold text-foreground">Ringkasan</h1>

      {stats.isError && (
        <Alert tone="error">Gagal memuat ringkasan hari ini. Coba muat ulang.</Alert>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Card title="Pesanan hari ini">
          <p className="text-2xl font-bold text-foreground">
            {s?.orders_count ?? (stats.isLoading ? '…' : '—')}
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            <Link to="/admin/pesanan" className="text-foreground hover:underline">
              Lihat pesanan
            </Link>
          </p>
        </Card>
        <Card title="Pendapatan hari ini">
          <p className="text-2xl font-bold text-foreground">
            {s ? formatIDR(s.revenue_paid) : stats.isLoading ? '…' : '—'}
          </p>
          <p className="mt-1 text-xs text-muted-foreground">Pesanan lunas hari ini</p>
        </Card>
        <Card title="Menunggu verifikasi">
          <p className="text-2xl font-bold text-foreground">
            {s?.awaiting_verification ?? (stats.isLoading ? '…' : '—')}
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            <Link to="/admin/pembayaran" className="text-foreground hover:underline">
              Verifikasi pembayaran
            </Link>
          </p>
        </Card>
        <Card title="Produk aktif">
          <p className="text-2xl font-bold text-foreground">
            {activeCount ?? (activeProducts.isLoading ? '…' : '—')}
          </p>
          <p className="mt-1 text-xs text-muted-foreground">
            <Link to="/admin/produk" className="text-foreground hover:underline">
              Kelola produk
            </Link>
          </p>
        </Card>
      </div>

      <Card
        title="Stok menipis"
        description="Produk yang stoknya sudah mencapai ambang minimum."
      >
        {stats.isLoading ? (
          <Spinner label="Memuat stok…" />
        ) : s && s.low_stock.length > 0 ? (
          <ul className="flex flex-col divide-y divide-border">
            {s.low_stock.map((p) => (
              <li key={p.product_id} className="flex items-center justify-between gap-3 py-2 text-sm">
                <span className="font-medium text-foreground">{p.name}</span>
                <span className="text-muted-foreground">Tersisa {formatQty(p.unit, p.stock)}</span>
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-sm text-muted-foreground">Semua stok masih aman.</p>
        )}
      </Card>
    </div>
  )
}
