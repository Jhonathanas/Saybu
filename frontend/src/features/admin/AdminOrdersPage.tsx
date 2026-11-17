import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Link } from 'react-router'
import { Card } from '@/components/ui/Card'
import { Alert, Spinner } from '@/components/ui/Feedback'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { ApiError } from '@/lib/api'
import { formatDateTime, formatIDR } from '@/lib/format'
import { adminListOrders } from '@/features/checkout/api'
import {
  fulfillmentLabel,
  paymentLabel,
  statusBadgeClass,
  statusLabel,
} from '@/features/checkout/status'
import { usePageTitle } from '@/lib/seo'

const STATUS_OPTIONS = [
  { value: '', label: 'Semua status' },
  { value: 'PENDING_PAYMENT', label: 'Menunggu pembayaran' },
  { value: 'WAITING_VERIFICATION', label: 'Menunggu verifikasi' },
  { value: 'PAID', label: 'Sudah dibayar' },
  { value: 'PREPARING', label: 'Sedang disiapkan' },
  { value: 'SHIPPED', label: 'Sedang dikirim' },
  { value: 'READY_PICKUP', label: 'Siap diambil' },
  { value: 'COMPLETED', label: 'Selesai' },
  { value: 'CANCELLED', label: 'Dibatalkan' },
  { value: 'EXPIRED', label: 'Kedaluwarsa' },
]

/** AdminOrdersPage menampilkan daftar pesanan dengan filter & pencarian. */
export function AdminOrdersPage() {
  usePageTitle('Pesanan · Admin')
  const [status, setStatus] = useState('')
  const [dateFrom, setDateFrom] = useState('')
  const [dateTo, setDateTo] = useState('')
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['admin', 'orders', { status, dateFrom, dateTo, query, page }],
    queryFn: () =>
      adminListOrders({
        status: status || undefined,
        date_from: dateFrom || undefined,
        date_to: dateTo || undefined,
        q: query || undefined,
        page,
        limit: 20,
      }),
  })

  const meta = data?.meta
  const totalPages = meta ? Math.max(1, Math.ceil(meta.total / meta.limit)) : 1
  const rows = data?.data ?? []

  const resetPage = () => setPage(1)

  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-xl font-bold text-stone-900">Pesanan</h1>

      <Card>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <Select
            label="Status"
            options={STATUS_OPTIONS}
            value={status}
            onChange={(e) => {
              setStatus(e.target.value)
              resetPage()
            }}
          />
          <Input
            label="Dari tanggal"
            type="date"
            value={dateFrom}
            onChange={(e) => {
              setDateFrom(e.target.value)
              resetPage()
            }}
          />
          <Input
            label="Sampai tanggal"
            type="date"
            value={dateTo}
            onChange={(e) => {
              setDateTo(e.target.value)
              resetPage()
            }}
          />
          <Input
            label="Cari"
            placeholder="Kode, nama, atau HP"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value)
              resetPage()
            }}
          />
        </div>
      </Card>

      {isLoading && <Spinner label="Memuat pesanan…" />}
      {isError && (
        <Alert tone="error">
          {error instanceof ApiError ? error.message : 'Gagal memuat pesanan.'}
        </Alert>
      )}

      {data && (
        <Card>
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="text-xs uppercase text-stone-500">
                <tr>
                  <th className="py-2 pr-3">Kode</th>
                  <th className="py-2 pr-3">Pelanggan</th>
                  <th className="py-2 pr-3">Total</th>
                  <th className="py-2 pr-3">Status</th>
                  <th className="py-2 pr-3">Dibuat</th>
                  <th className="py-2">Aksi</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-stone-100">
                {rows.map((o) => (
                  <tr key={o.id}>
                    <td className="py-2 pr-3">
                      <Link
                        to={`/admin/pesanan/${o.id}`}
                        className="font-mono font-medium text-brand-700 hover:underline"
                      >
                        {o.code}
                      </Link>
                      <span className="block text-xs text-stone-500">
                        {fulfillmentLabel(o.fulfillment)} · {paymentLabel(o.payment_method)}
                      </span>
                    </td>
                    <td className="py-2 pr-3">
                      <span className="block text-stone-900">{o.customer.name}</span>
                      <span className="block text-xs text-stone-500">{o.customer.phone}</span>
                    </td>
                    <td className="py-2 pr-3 whitespace-nowrap text-stone-900">
                      {formatIDR(o.total)}
                    </td>
                    <td className="py-2 pr-3">
                      <span
                        className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset ${statusBadgeClass(o.status)}`}
                      >
                        {statusLabel(o.status)}
                      </span>
                    </td>
                    <td className="py-2 pr-3 whitespace-nowrap text-stone-600">
                      {formatDateTime(o.created_at)}
                    </td>
                    <td className="py-2">
                      <Link
                        to={`/admin/pesanan/${o.id}`}
                        className="tap-target rounded-lg px-2 ring-1 ring-stone-300 hover:bg-stone-100"
                      >
                        Detail
                      </Link>
                    </td>
                  </tr>
                ))}
                {rows.length === 0 && (
                  <tr>
                    <td colSpan={6} className="py-6 text-center text-stone-500">
                      Tidak ada pesanan pada filter ini.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          <div className="mt-4 flex items-center justify-between text-sm text-stone-600">
            <span>
              Halaman {meta?.page ?? 1} dari {totalPages} · total {meta?.total ?? 0}
            </span>
            <div className="flex gap-2">
              <button
                type="button"
                className="tap-target rounded-lg px-3 ring-1 ring-stone-300 disabled:opacity-40"
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                Sebelumnya
              </button>
              <button
                type="button"
                className="tap-target rounded-lg px-3 ring-1 ring-stone-300 disabled:opacity-40"
                disabled={page >= totalPages}
                onClick={() => setPage((p) => p + 1)}
              >
                Berikutnya
              </button>
            </div>
          </div>
        </Card>
      )}
    </div>
  )
}
