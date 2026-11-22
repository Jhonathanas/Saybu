import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { toast } from 'sonner'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Alert, Spinner } from '@/components/ui/Feedback'
import { Select } from '@/components/ui/Select'
import { ApiError } from '@/lib/api'
import { formatDateTime, formatIDR } from '@/lib/format'
import { listPayments, verifyPayment } from '@/features/payment/api'
import {
  paymentStatusBadgeClass,
  paymentStatusLabel,
  type AdminPayment,
} from '@/features/payment/types'
import { usePageTitle } from '@/lib/seo'

const STATUS_OPTIONS = [
  { value: 'WAITING_VERIFICATION', label: 'Menunggu verifikasi' },
  { value: 'UNPAID', label: 'Belum dibayar' },
  { value: 'PAID', label: 'Sudah dibayar' },
  { value: 'REJECTED', label: 'Bukti ditolak' },
  { value: 'COD_PENDING', label: 'COD — belum dibayar' },
  { value: 'COD_PAID', label: 'COD — sudah dibayar' },
  { value: 'CANCELLED', label: 'Dibatalkan' },
  { value: '', label: 'Semua status' },
]

/** AdminPaymentsPage memverifikasi bukti transfer dan memantau pembayaran. */
export function AdminPaymentsPage() {
  usePageTitle('Pembayaran · Admin')
  const queryClient = useQueryClient()
  const [status, setStatus] = useState('WAITING_VERIFICATION')
  const [page, setPage] = useState(1)
  const [rejecting, setRejecting] = useState<{ id: string; note: string } | null>(null)

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['admin', 'payments', { status, page }],
    queryFn: () => listPayments({ status: status || undefined, page, limit: 20 }),
  })

  const verify = useMutation({
    mutationFn: ({ id, decision, note }: { id: string; decision: 'APPROVE' | 'REJECT'; note: string }) =>
      verifyPayment(id, decision, note),
    onSuccess: (updated: AdminPayment, variables) => {
      setRejecting(null)
      toast.success(
        variables.decision === 'APPROVE'
          ? `Pembayaran ${updated.order_code} disetujui.`
          : `Pembayaran ${updated.order_code} ditolak.`,
      )
      void queryClient.invalidateQueries({ queryKey: ['admin', 'payments'] })
      void queryClient.invalidateQueries({ queryKey: ['order', updated.order_code] })
    },
    onError: (err) => {
      toast.error(err instanceof ApiError ? err.message : 'Gagal memverifikasi pembayaran.')
    },
  })

  const meta = data?.meta
  const totalPages = meta ? Math.max(1, Math.ceil(meta.total / meta.limit)) : 1
  const rows = data?.data ?? []

  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-xl font-bold text-foreground">Pembayaran</h1>

      <Card>
        <div className="max-w-sm">
          <Select
            label="Filter status"
            options={STATUS_OPTIONS}
            value={status}
            onChange={(e) => {
              setStatus(e.target.value)
              setPage(1)
            }}
          />
        </div>
      </Card>

      {isLoading && <Spinner label="Memuat pembayaran…" />}
      {isError && (
        <Alert tone="error">
          {error instanceof ApiError ? error.message : 'Gagal memuat pembayaran.'}
        </Alert>
      )}

      {data && (
        <Card>
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="text-xs uppercase text-muted-foreground">
                <tr>
                  <th className="py-2 pr-3">Kode</th>
                  <th className="py-2 pr-3">Pelanggan</th>
                  <th className="py-2 pr-3">Metode</th>
                  <th className="py-2 pr-3">Nominal</th>
                  <th className="py-2 pr-3">Status</th>
                  <th className="py-2 pr-3">Bukti</th>
                  <th className="py-2">Aksi</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {rows.map((p) => (
                  <PaymentRow
                    key={p.id}
                    payment={p}
                    rejecting={rejecting}
                    onReject={setRejecting}
                    onApprove={(id) => verify.mutate({ id, decision: 'APPROVE', note: '' })}
                    onConfirmReject={(id, note) => verify.mutate({ id, decision: 'REJECT', note })}
                    submitting={verify.isPending}
                  />
                ))}
                {rows.length === 0 && (
                  <tr>
                    <td colSpan={7} className="py-6 text-center text-muted-foreground">
                      Tidak ada pembayaran pada status ini.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          <div className="mt-4 flex items-center justify-between text-sm text-muted-foreground">
            <span>
              Halaman {meta?.page ?? 1} dari {totalPages} · total {meta?.total ?? 0}
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
        </Card>
      )}
    </div>
  )
}

interface PaymentRowProps {
  payment: AdminPayment
  rejecting: { id: string; note: string } | null
  onReject: (value: { id: string; note: string }) => void
  onApprove: (id: string) => void
  onConfirmReject: (id: string, note: string) => void
  submitting: boolean
}

function PaymentRow({
  payment,
  rejecting,
  onReject,
  onApprove,
  onConfirmReject,
  submitting,
}: PaymentRowProps) {
  const isRejecting = rejecting?.id === payment.id
  const canVerify = payment.status === 'WAITING_VERIFICATION'

  return (
    <>
      <tr>
        <td className="py-2 pr-3 font-mono font-medium text-foreground">{payment.order_code}</td>
        <td className="py-2 pr-3">
          <span className="block text-foreground">{payment.customer_name || '—'}</span>
          <span className="block text-xs text-muted-foreground">{payment.customer_phone || '—'}</span>
        </td>
        <td className="py-2 pr-3">{payment.method}</td>
        <td className="py-2 pr-3 text-foreground">{formatIDR(payment.amount)}</td>
        <td className="py-2 pr-3">
          <span
            className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset ${paymentStatusBadgeClass(payment.status)}`}
          >
            {paymentStatusLabel(payment.status)}
          </span>
        </td>
        <td className="py-2 pr-3">
          {payment.proof_url ? (
            <a
              href={payment.proof_url}
              target="_blank"
              rel="noreferrer"
              className="font-medium text-foreground hover:underline"
            >
              Lihat bukti
            </a>
          ) : (
            <span className="text-muted-foreground">—</span>
          )}
          {payment.uploaded_at && (
            <span className="block text-xs text-muted-foreground">
              {formatDateTime(payment.uploaded_at)}
            </span>
          )}
        </td>
        <td className="py-2">
          {canVerify ? (
            <div className="flex flex-wrap gap-2">
              <Button size="sm" loading={submitting && !isRejecting} onClick={() => onApprove(payment.id)}>
                Setujui
              </Button>
              <Button
                size="sm"
                variant="danger"
                onClick={() => onReject({ id: payment.id, note: '' })}
              >
                Tolak
              </Button>
            </div>
          ) : (
            <span className="text-muted-foreground">—</span>
          )}
        </td>
      </tr>
      {isRejecting && (
        <tr>
          <td colSpan={7} className="bg-destructive/10 py-3">
            <div className="flex flex-col gap-2">
              <label
                htmlFor={`reject-${payment.id}`}
                className="text-sm font-medium text-destructive"
              >
                Alasan penolakan (dikirim ke pelanggan)
              </label>
              <textarea
                id={`reject-${payment.id}`}
                rows={2}
                className="tap-target w-full rounded-lg border border-destructive/30 bg-card px-3 py-2 text-sm text-foreground"
                value={rejecting.note}
                onChange={(e) => onReject({ id: payment.id, note: e.target.value })}
              />
              <div className="flex gap-2">
                <Button
                  size="sm"
                  variant="danger"
                  loading={submitting}
                  disabled={rejecting.note.trim().length === 0}
                  onClick={() => onConfirmReject(payment.id, rejecting.note.trim())}
                >
                  Kirim penolakan
                </Button>
                <Button size="sm" variant="secondary" onClick={() => onReject({ id: '', note: '' })}>
                  Batal
                </Button>
              </div>
            </div>
          </td>
        </tr>
      )}
    </>
  )
}
