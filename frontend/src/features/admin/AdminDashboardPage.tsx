import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
import { Card } from '@/components/ui/Card'
import { listProducts } from '@/features/catalog/api'
import { listPayments } from '@/features/payment/api'

export function AdminDashboardPage() {
  const activeProducts = useQuery({
    queryKey: ['products', { page: 1, limit: 1 }],
    queryFn: () => listProducts({ page: 1, limit: 1 }),
  })

  const pendingPayments = useQuery({
    queryKey: ['admin', 'payments', { status: 'WAITING_VERIFICATION', page: 1 }],
    queryFn: () => listPayments({ status: 'WAITING_VERIFICATION', page: 1, limit: 1 }),
  })

  const activeCount = activeProducts.data?.meta?.total
  const pendingCount = pendingPayments.data?.meta?.total

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-xl font-bold text-stone-900">Ringkasan</h1>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Card title="Pesanan hari ini">
          <p className="text-2xl font-bold text-stone-900">—</p>
          <p className="mt-1 text-xs text-stone-500">Tersedia pada milestone M5.</p>
        </Card>
        <Card title="Menunggu verifikasi">
          <p className="text-2xl font-bold text-stone-900">
            {pendingCount ?? (pendingPayments.isLoading ? '…' : '—')}
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

      <Card title="Selamat datang" description="Panel admin Saybu">
        <p className="text-sm text-stone-600">
          Gunakan menu di samping untuk mengelola produk, memverifikasi pembayaran, dan
          mengelola pengguna. Menu pesanan akan muncul seiring bertambahnya milestone.
        </p>
      </Card>
    </div>
  )
}
