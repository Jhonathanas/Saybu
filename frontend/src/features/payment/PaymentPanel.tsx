import { useQuery, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Alert } from '@/components/ui/Feedback'
import { CopyButton } from '@/components/ui/CopyButton'
import type { Order } from '@/features/checkout/types'
import { BANK } from '@/lib/env'
import { formatDateTime, formatIDR } from '@/lib/format'
import { getPayment } from './api'
import { ProofUploadForm } from './ProofUploadForm'
import { canUploadProof, paymentStatusBadgeClass, paymentStatusLabel } from './types'
import type { BankInfo, Payment } from './types'

interface PaymentPanelProps {
  order: Order
  /** Nomor HP pemesan untuk pesanan guest (opsional bila sudah login). */
  phone?: string
}

/**
 * PaymentPanel menampilkan status & instruksi pembayaran serta form unggah
 * bukti transfer. Informasi rekening diambil dari API (fallback ke env).
 */
export function PaymentPanel({ order, phone }: PaymentPanelProps) {
  const queryClient = useQueryClient()
  const queryKey = ['payment', order.code, phone ?? '']

  const paymentQuery = useQuery({
    queryKey,
    queryFn: () => getPayment(order.code, phone),
    retry: false,
  })

  const payment = paymentQuery.data
  const transfer = order.payment_method === 'TRANSFER'

  function handleUploaded(updated: Payment) {
    queryClient.setQueryData(queryKey, updated)
    void queryClient.invalidateQueries({ queryKey: ['order', order.code] })
    toast.success('Bukti transfer terkirim. Menunggu verifikasi admin.')
  }

  // Selama memuat atau bila gagal, jangan tampilkan apa pun: rincian pesanan
  // sudah memberi konteks yang cukup.
  if (!payment) return null

  // Informasi rekening dari API; fallback ke env Vite bila API tidak mengirim.
  const bank: BankInfo | null = payment.bank
    ? payment.bank
    : transfer
      ? { name: BANK.name, account_number: BANK.account, account_holder: BANK.holder }
      : null
  const uploadable = transfer && canUploadProof(payment.status)
  const showBank = transfer && uploadable && bank !== null

  return (
    <section className="rounded-2xl border border-border bg-card p-4">
      <div className="flex flex-wrap items-center justify-between gap-2">
        <h2 className="font-semibold text-foreground">Pembayaran</h2>
        <span
          className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset ${paymentStatusBadgeClass(payment.status)}`}
        >
          {paymentStatusLabel(payment.status)}
        </span>
      </div>
      <p className="mt-1 text-sm text-muted-foreground">Nominal {formatIDR(payment.amount)}</p>

      {payment.status === 'PAID' && (
        <div className="mt-3">
          <Alert tone="success">
            Pembayaran sudah diverifikasi
            {payment.verified_at ? ` pada ${formatDateTime(payment.verified_at)}` : ''}.
          </Alert>
        </div>
      )}

      {payment.status === 'WAITING_VERIFICATION' && (
        <div className="mt-3">
          <Alert tone="info">
            Bukti transfer sudah kami terima dan sedang diverifikasi admin.
          </Alert>
        </div>
      )}

      {payment.status === 'REJECTED' && (
        <div className="mt-3">
          <Alert tone="error">
            Bukti transfer ditolak{payment.note ? `: ${payment.note}` : ''}. Silakan unggah ulang
            bukti yang benar.
          </Alert>
        </div>
      )}

      {showBank && bank && (
        <div className="mt-3 rounded-xl border border-border bg-muted p-3">
          <p className="text-sm text-foreground">
            Transfer tepat sebesar <strong>{formatIDR(payment.amount)}</strong> ke rekening berikut,
            lalu unggah bukti transfer di bawah.
          </p>
          <dl className="mt-2 flex flex-col gap-1.5 text-sm">
            <div className="flex items-center justify-between gap-3">
              <dt className="text-foreground">Bank</dt>
              <dd className="font-medium text-foreground">{bank.name}</dd>
            </div>
            <div className="flex items-center justify-between gap-3">
              <dt className="text-foreground">Nomor rekening</dt>
              <dd className="flex items-center gap-2 font-mono font-medium text-foreground">
                {bank.account_number}
                <CopyButton value={bank.account_number} />
              </dd>
            </div>
            <div className="flex items-center justify-between gap-3">
              <dt className="text-foreground">Atas nama</dt>
              <dd className="font-medium text-foreground">{bank.account_holder}</dd>
            </div>
            <div className="flex items-center justify-between gap-3">
              <dt className="text-foreground">Jumlah</dt>
              <dd className="flex items-center gap-2 font-medium text-foreground">
                {formatIDR(payment.amount)}
                <CopyButton value={String(payment.amount)} />
              </dd>
            </div>
          </dl>
          <p className="mt-2 text-xs text-foreground">
            Sertakan kode <span className="font-mono font-semibold">{order.code}</span> pada berita
            transfer agar admin mudah memverifikasi.
          </p>
        </div>
      )}

      {!transfer && (
        <div className="mt-3">
          <Alert tone="info">
            Pesanan COD — bayar saat pesanan diterima/diambil. Tidak perlu transfer.
          </Alert>
        </div>
      )}

      {uploadable && (
        <div className="mt-4 border-t border-border pt-4">
          <ProofUploadForm code={order.code} phone={phone} onUploaded={handleUploaded} />
        </div>
      )}
    </section>
  )
}
