import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router'
import { toast } from 'sonner'
import { Button } from '@/components/ui/Button'
import { Alert, Spinner } from '@/components/ui/Feedback'
import { useAuth } from '@/features/auth/AuthProvider'
import { PaymentPanel } from '@/features/payment/PaymentPanel'
import { ApiError } from '@/lib/api'
import { usePageTitle } from '@/lib/seo'
import { cancelOrder, getOrder } from './api'
import { OrderDetailView } from './OrderDetailView'
import { isCancellable } from './status'

export function OrderDetailPage() {
  usePageTitle('Detail pesanan')
  const { code = '' } = useParams()
  const [searchParams] = useSearchParams()
  const phone = searchParams.get('phone') ?? undefined
  const isNew = searchParams.get('baru') === '1'
  const { user, loading: authLoading } = useAuth()
  const queryClient = useQueryClient()

  const [confirming, setConfirming] = useState(false)
  const [cancelling, setCancelling] = useState(false)
  const [cancelError, setCancelError] = useState<string | null>(null)

  const canFetch = !!code && (!!phone || !!user)
  const orderQuery = useQuery({
    queryKey: ['order', code, phone ?? ''],
    queryFn: () => getOrder(code, phone),
    enabled: canFetch,
    retry: false,
  })

  async function handleCancel() {
    setCancelling(true)
    setCancelError(null)
    try {
      const updated = await cancelOrder(code, phone, 'Dibatalkan oleh pembeli')
      queryClient.setQueryData(['order', code, phone ?? ''], updated)
      toast.success('Pesanan dibatalkan')
      setConfirming(false)
    } catch (err) {
      if (err instanceof ApiError) {
        setCancelError(err.message)
      } else {
        setCancelError('Gagal membatalkan pesanan. Coba lagi.')
      }
    } finally {
      setCancelling(false)
    }
  }

  if (!canFetch) {
    if (authLoading) return <Spinner />
    return (
      <div className="flex flex-col gap-4">
        <h1 className="text-xl font-bold text-foreground">Pesanan</h1>
        <Alert tone="info">
          Untuk melacak pesanan tanpa akun, masukkan kode pesanan dan nomor HP di{' '}
          <Link to="/pesanan" className="font-medium underline">
            halaman lacak
          </Link>
          .
        </Alert>
      </div>
    )
  }

  if (orderQuery.isLoading) return <Spinner label="Memuat pesanan…" />

  if (orderQuery.isError) {
    const err = orderQuery.error
    const notFound = err instanceof ApiError && err.status === 404
    return (
      <div className="flex flex-col gap-4">
        <h1 className="text-xl font-bold text-foreground">Pesanan</h1>
        <Alert tone="error">
          {notFound
            ? 'Pesanan tidak ditemukan atau nomor HP tidak cocok.'
            : 'Gagal memuat pesanan. Coba lagi.'}
        </Alert>
        <Link to="/pesanan" className="text-sm font-medium text-foreground hover:underline">
          ← Lacak pesanan lain
        </Link>
      </div>
    )
  }

  const order = orderQuery.data!

  return (
    <div className="flex flex-col gap-5">
      {isNew && (
        <Alert tone="success">
          Pesanan berhasil dibuat! Simpan kode <strong>{order.code}</strong> untuk melacak status.
        </Alert>
      )}

      <h1 className="text-xl font-bold text-foreground">Detail pesanan</h1>

      <OrderDetailView
        order={order}
        onCancel={
          isCancellable(order.status) && !confirming
            ? () => setConfirming(true)
            : undefined
        }
        cancelling={cancelling}
        cancelError={cancelError}
      />

      <PaymentPanel order={order} phone={phone} />

      {confirming && (
        <div className="flex flex-col gap-3 rounded-2xl border border-destructive/30 bg-destructive/10 p-4">
          <p className="text-sm text-destructive">
            Yakin ingin membatalkan pesanan ini? Stok akan dikembalikan.
          </p>
          <div className="flex gap-2">
            <Button variant="danger" loading={cancelling} onClick={handleCancel}>
              Ya, batalkan
            </Button>
            <Button variant="secondary" disabled={cancelling} onClick={() => setConfirming(false)}>
              Tidak
            </Button>
          </div>
        </div>
      )}

      <Link to="/pesanan" className="text-sm font-medium text-foreground hover:underline">
        ← Lacak pesanan lain
      </Link>
    </div>
  )
}
