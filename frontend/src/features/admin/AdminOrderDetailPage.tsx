import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { Link, useParams } from 'react-router'
import { toast } from 'sonner'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Alert, Spinner } from '@/components/ui/Feedback'
import { Select } from '@/components/ui/Select'
import { ApiError } from '@/lib/api'
import { adminGetOrder, adminUpdateOrderStatus } from '@/features/checkout/api'
import { OrderDetailView } from '@/features/checkout/OrderDetailView'
import { allowedTransitions, statusLabel } from '@/features/checkout/status'
import { usePageTitle } from '@/lib/seo'

/** AdminOrderDetailPage menampilkan rincian pesanan & mengubah statusnya. */
export function AdminOrderDetailPage() {
  usePageTitle('Detail pesanan · Admin')
  const { id = '' } = useParams()
  const queryClient = useQueryClient()

  const orderQuery = useQuery({
    queryKey: ['admin', 'order', id],
    queryFn: () => adminGetOrder(id),
    enabled: !!id,
  })

  const order = orderQuery.data

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between gap-3">
        <h1 className="text-xl font-bold text-foreground">Detail pesanan</h1>
        <Link to="/admin/pesanan" className="text-sm text-muted-foreground hover:underline">
          ← Daftar pesanan
        </Link>
      </div>

      {orderQuery.isLoading && <Spinner label="Memuat pesanan…" />}
      {orderQuery.isError && (
        <Alert tone="error">
          {orderQuery.error instanceof ApiError && orderQuery.error.status === 404
            ? 'Pesanan tidak ditemukan.'
            : 'Gagal memuat pesanan.'}
        </Alert>
      )}

      {order && (
        <>
          <OrderDetailView order={order} />
          <StatusForm
            orderId={order.id}
            status={order.status}
            onUpdated={() => {
              void queryClient.invalidateQueries({ queryKey: ['admin', 'order', id] })
              void queryClient.invalidateQueries({ queryKey: ['admin', 'orders'] })
              void queryClient.invalidateQueries({ queryKey: ['admin', 'stats'] })
            }}
          />
        </>
      )}
    </div>
  )
}

interface StatusFormProps {
  orderId: string
  status: string
  onUpdated: () => void
}

function StatusForm({ orderId, status, onUpdated }: StatusFormProps) {
  const transitions = allowedTransitions(status)
  const [to, setTo] = useState(transitions[0] ?? '')
  const [note, setNote] = useState('')

  // Setel ulang pilihan ketika status pesanan berubah (mis. setelah update).
  useEffect(() => {
    setTo(allowedTransitions(status)[0] ?? '')
    setNote('')
  }, [status])

  const update = useMutation({
    mutationFn: () => adminUpdateOrderStatus(orderId, to, note.trim()),
    onSuccess: () => {
      toast.success(`Status diubah ke ${statusLabel(to)}.`)
      onUpdated()
    },
    onError: (err) => {
      toast.error(err instanceof ApiError ? err.message : 'Gagal mengubah status.')
    },
  })

  if (transitions.length === 0) {
    return (
      <Card title="Ubah status">
        <p className="text-sm text-muted-foreground">
          Status <strong>{statusLabel(status)}</strong> bersifat final dan tidak bisa diubah.
        </p>
      </Card>
    )
  }

  return (
    <Card title="Ubah status" description={`Status saat ini: ${statusLabel(status)}`}>
      <form
        className="flex flex-col gap-4"
        onSubmit={(e) => {
          e.preventDefault()
          if (to) update.mutate()
        }}
      >
        <Select
          label="Status baru"
          value={to}
          onChange={(e) => setTo(e.target.value)}
          options={transitions.map((s) => ({ value: s, label: statusLabel(s) }))}
        />

        <div className="flex flex-col gap-1.5">
          <label htmlFor="catatan-status" className="text-sm font-medium text-foreground">
            Catatan (opsional)
          </label>
          <textarea
            id="catatan-status"
            rows={2}
            value={note}
            onChange={(e) => setNote(e.target.value)}
            placeholder="Mis. sedang disiapkan pagi ini"
            className="w-full rounded-lg border border-border bg-card px-3 py-2 text-sm text-foreground"
          />
        </div>

        <div>
          <Button type="submit" loading={update.isPending} disabled={!to}>
            Simpan status
          </Button>
        </div>
      </form>
    </Card>
  )
}
