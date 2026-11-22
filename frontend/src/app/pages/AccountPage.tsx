import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Link } from 'react-router'
import { Card } from '@/components/ui/Card'
import { Alert, Spinner } from '@/components/ui/Feedback'
import { useAuth } from '@/features/auth/AuthProvider'
import { listMyOrders } from '@/features/checkout/api'
import { statusBadgeClass, statusLabel } from '@/features/checkout/status'
import { ApiError } from '@/lib/api'
import { formatDateTime, formatIDR } from '@/lib/format'
import { usePageTitle } from '@/lib/seo'

export function AccountPage() {
  usePageTitle('Akun Saya')
  const { user } = useAuth()
  const [page, setPage] = useState(1)

  const orders = useQuery({
    queryKey: ['me', 'orders', { page }],
    queryFn: () => listMyOrders({ page, limit: 10 }),
  })

  if (!user) return null

  const meta = orders.data?.meta
  const totalPages = meta ? Math.max(1, Math.ceil(meta.total / meta.limit)) : 1
  const rows = orders.data?.data ?? []

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-xl font-bold text-foreground">Akun Saya</h1>

      <Card title="Profil">
        <dl className="grid gap-3 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-muted-foreground">Nama</dt>
            <dd className="font-medium text-foreground">{user.name}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">Nomor HP</dt>
            <dd className="font-medium text-foreground">{user.phone}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">Email</dt>
            <dd className="font-medium text-foreground">{user.email || '—'}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">Peran</dt>
            <dd className="font-medium text-foreground">{user.role}</dd>
          </div>
          {user.created_at && (
            <div>
              <dt className="text-muted-foreground">Terdaftar</dt>
              <dd className="font-medium text-foreground">{formatDateTime(user.created_at)}</dd>
            </div>
          )}
        </dl>
      </Card>

      <Card title="Riwayat pesanan">
        {orders.isLoading && <Spinner label="Memuat pesanan…" />}
        {orders.isError && (
          <Alert tone="error">
            {orders.error instanceof ApiError ? orders.error.message : 'Gagal memuat pesanan.'}
          </Alert>
        )}

        {orders.data && (
          <>
            {rows.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                Belum ada pesanan.{' '}
                <Link to="/produk" className="font-medium text-foreground hover:underline">
                  Mulai belanja
                </Link>
                .
              </p>
            ) : (
              <ul className="flex flex-col divide-y divide-border">
                {rows.map((o) => (
                  <li key={o.id} className="flex items-center justify-between gap-3 py-3">
                    <div>
                      <Link
                        to={`/pesanan/${o.code}`}
                        className="font-mono font-medium text-foreground hover:underline"
                      >
                        {o.code}
                      </Link>
                      <p className="text-xs text-muted-foreground">{formatDateTime(o.created_at)}</p>
                    </div>
                    <div className="flex flex-col items-end gap-1">
                      <span
                        className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset ${statusBadgeClass(o.status)}`}
                      >
                        {statusLabel(o.status)}
                      </span>
                      <span className="text-sm text-foreground">{formatIDR(o.total)}</span>
                    </div>
                  </li>
                ))}
              </ul>
            )}

            {totalPages > 1 && (
              <div className="mt-4 flex items-center justify-between text-sm text-muted-foreground">
                <span>
                  Halaman {meta?.page ?? 1} dari {totalPages}
                </span>
                <div className="flex gap-2">
                  <button
                    type="button"
                    className="tap-target rounded-lg px-3 ring-1 ring-border disabled:opacity-40"
                    disabled={page <= 1}
                    onClick={() => setPage((p) => Math.max(1, p - 1))}
                  >
                    Sebelumnya
                  </button>
                  <button
                    type="button"
                    className="tap-target rounded-lg px-3 ring-1 ring-border disabled:opacity-40"
                    disabled={page >= totalPages}
                    onClick={() => setPage((p) => p + 1)}
                  >
                    Berikutnya
                  </button>
                </div>
              </div>
            )}
          </>
        )}
      </Card>
    </div>
  )
}
