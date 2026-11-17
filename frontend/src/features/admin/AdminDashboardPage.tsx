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
      <h1 className="text-xl font-bold text-stone-900">Ringkasan</h1>

      {stats.isError && (
        <Alert tone="error">Gagal memuat ringkasan hari ini. Coba muat ulang.</Alert>
      )}

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
        <Card title="Pesanan hari ini">
          <p className="text-2xl font-bold text-stone-900">
            {s?.orders_count ?? (stats.isLoading ? '…' : '—')}
          </p>
          <p className="mt-1 text-xs text-stone-500">
            <Link to="/admin/pesanan" className="text-brand-700 hover:underline">
              Lihat pesanan
            </Link>
          </p>
        </Card>
        <Card title="Pendapatan hari ini">
          <p className="text-2xl font-bold text-stone-900">
            {s ? formatIDR(s.revenue_paid) : stats.isLoading ? '…' : '—'}
          </p>
          <p className="mt-1 text-xs text-stone-500">Pesanan lunas hari ini</p>
        </Card>
        <Card title="Menunggu verifikasi">
          <p className="text-2xl font-bold text-stone-900">
            {s?.awaiting_verification ?? (stats.isLoading ? '…' : '—')}
          </p>
          <p className="mt-1 text-xs text-stone-500">
            <Link to="/admin/pembayaran" className="text-brand-700 hover:underline">
              Verifikasi pembayaran
            </Link>
          </p>
        </Card>
        <Card title="Produk aktif">
          <p className="text-2xl font-bold text-stone-900">
            {activeCount ?? (activeProducts.isLoading ? '…' : '—')}
          </p>
          <p className="mt-1 text-xs text-stone-500">
            <Link to="/admin/produk" className="text-brand-700 hover:underline">
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
          <ul className="flex flex-col divide-y divide-stone-100">
            {s.low_stock.map((p) => (
              <li key={p.product_id} className="flex items-center justify-between gap-3 py-2 text-sm">
                <span className="font-medium text-stone-900">{p.name}</span>
                <span className="text-amber-700">Tersisa {formatQty(p.unit, p.stock)}</span>
              </li>
            ))}
          </ul>
        ) : (
          <p className="text-sm text-stone-500">Semua stok masih aman.</p>
        )}
      </Card>
    </div>
  )
}
