import { useState } from 'react'
import { toast } from 'sonner'
import { Alert } from '@/components/ui/Feedback'
import { Button } from '@/components/ui/Button'
import { BANK } from '@/lib/env'
import { formatDateTime, formatIDR } from '@/lib/format'
import {
  fulfillmentLabel,
  isUnpaid,
  paymentLabel,
  statusBadgeClass,
  statusLabel,
} from './status'
import type { Order } from './types'

/** StatusBadge menampilkan status pesanan dengan warna sesuai tahapannya. */
export function StatusBadge({ status }: { status: string }) {
  return (
    <span
      className={`inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset ${statusBadgeClass(status)}`}
    >
      {statusLabel(status)}
    </span>
  )
}

/** CopyButton menyalin teks ke papan klip dengan umpan balik toast. */
export function CopyButton({ value, label = 'Salin' }: { value: string; label?: string }) {
  const [copied, setCopied] = useState(false)

  async function copy() {
    try {
      await navigator.clipboard.writeText(value)
      setCopied(true)
      toast.success('Disalin ke papan klip')
      window.setTimeout(() => setCopied(false), 2000)
    } catch {
      toast.error('Gagal menyalin. Salin manual ya.')
    }
  }

  return (
    <Button type="button" variant="secondary" size="sm" onClick={copy}>
      {copied ? 'Tersalin ✓' : label}
    </Button>
  )
}

interface OrderDetailViewProps {
  order: Order
  /** Tampilkan instruksi pembayaran transfer bila belum dibayar. */
  showPaymentInstructions?: boolean
  onCancel?: () => void
  cancelling?: boolean
  cancelError?: string | null
}

/** OrderDetailView merender rincian pesanan (dipakai halaman sukses & lacak). */
export function OrderDetailView({
  order,
  showPaymentInstructions = false,
  onCancel,
  cancelling = false,
  cancelError,
}: OrderDetailViewProps) {
  const transfer = order.payment_method === 'TRANSFER'
  const showBank = showPaymentInstructions && transfer && isUnpaid(order.status)

  return (
    <div className="flex flex-col gap-5">
      <section className="rounded-2xl border border-stone-200 bg-white p-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <p className="text-xs text-stone-500">Kode pesanan</p>
            <p className="font-mono text-lg font-bold text-stone-900">{order.code}</p>
          </div>
          <div className="flex items-center gap-2">
            <StatusBadge status={order.status} />
            <CopyButton value={order.code} label="Salin kode" />
          </div>
        </div>
        <dl className="mt-4 grid gap-2 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-stone-500">Dibuat</dt>
            <dd className="text-stone-900">{formatDateTime(order.created_at)}</dd>
          </div>
          <div>
            <dt className="text-stone-500">
              {isUnpaid(order.status) ? 'Batas pembayaran' : 'Status'}
            </dt>
            <dd className="text-stone-900">
              {isUnpaid(order.status) ? formatDateTime(order.expires_at) : statusLabel(order.status)}
            </dd>
          </div>
        </dl>
        {order.cancel_reason && (
          <p className="mt-2 text-sm text-red-600">Alasan batal: {order.cancel_reason}</p>
        )}
      </section>

      {showBank && (
        <section className="rounded-2xl border border-brand-200 bg-brand-50 p-4">
          <h2 className="font-semibold text-brand-900">Instruksi pembayaran</h2>
          <p className="mt-1 text-sm text-brand-800">
            Transfer tepat sebesar <strong>{formatIDR(order.total)}</strong> ke rekening berikut,
            lalu konfirmasi ke admin via WhatsApp.
          </p>
          <dl className="mt-3 flex flex-col gap-2 text-sm">
            <div className="flex items-center justify-between gap-3">
              <dt className="text-brand-800">Bank</dt>
              <dd className="font-medium text-brand-900">{BANK.name}</dd>
            </div>
            <div className="flex items-center justify-between gap-3">
              <dt className="text-brand-800">Nomor rekening</dt>
              <dd className="flex items-center gap-2 font-mono font-medium text-brand-900">
                {BANK.account}
                <CopyButton value={BANK.account} />
              </dd>
            </div>
            <div className="flex items-center justify-between gap-3">
              <dt className="text-brand-800">Atas nama</dt>
              <dd className="font-medium text-brand-900">{BANK.holder}</dd>
            </div>
            <div className="flex items-center justify-between gap-3">
              <dt className="text-brand-800">Jumlah</dt>
              <dd className="flex items-center gap-2 font-medium text-brand-900">
                {formatIDR(order.total)}
                <CopyButton value={String(order.total)} />
              </dd>
            </div>
          </dl>
          <p className="mt-3 text-xs text-brand-800">
            Sertakan kode <span className="font-mono font-semibold">{order.code}</span> pada berita
            transfer agar admin mudah memverifikasi.
          </p>
        </section>
      )}

      {showPaymentInstructions && !transfer && order.status === 'PENDING_PAYMENT' && (
        <Alert tone="info">
          Pesanan COD — bayar saat pesanan diterima/diambil. Tidak perlu transfer.
        </Alert>
      )}

      <section className="rounded-2xl border border-stone-200 bg-white p-4">
        <h2 className="font-semibold text-stone-900">Produk</h2>
        <ul className="mt-3 flex flex-col divide-y divide-stone-100">
          {order.items.map((item) => (
            <li key={item.product_id} className="flex items-start justify-between gap-3 py-2">
              <div>
                <p className="font-medium text-stone-900">{item.name}</p>
                <p className="text-sm text-stone-500">
                  {item.qty_display} × {formatIDR(item.unit_price)}
                </p>
              </div>
              <p className="font-medium text-stone-900">{formatIDR(item.line_total)}</p>
            </li>
          ))}
        </ul>
        <dl className="mt-3 flex flex-col gap-1.5 border-t border-stone-100 pt-3 text-sm">
          <div className="flex justify-between">
            <dt className="text-stone-500">Subtotal</dt>
            <dd className="text-stone-900">{formatIDR(order.subtotal)}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-stone-500">Ongkir</dt>
            <dd className="text-stone-900">
              {order.shipping_fee === 0 ? 'Gratis' : formatIDR(order.shipping_fee)}
            </dd>
          </div>
          <div className="flex justify-between text-base font-bold">
            <dt>Total</dt>
            <dd>{formatIDR(order.total)}</dd>
          </div>
        </dl>
      </section>

      <section className="rounded-2xl border border-stone-200 bg-white p-4">
        <h2 className="font-semibold text-stone-900">Pengiriman</h2>
        <dl className="mt-3 grid gap-2 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-stone-500">Cara</dt>
            <dd className="text-stone-900">{fulfillmentLabel(order.fulfillment)}</dd>
          </div>
          <div>
            <dt className="text-stone-500">Pembayaran</dt>
            <dd className="text-stone-900">{paymentLabel(order.payment_method)}</dd>
          </div>
          {order.zone_name && (
            <div>
              <dt className="text-stone-500">Zona</dt>
              <dd className="text-stone-900">{order.zone_name}</dd>
            </div>
          )}
          <div>
            <dt className="text-stone-500">Penerima</dt>
            <dd className="text-stone-900">
              {order.customer.name} · {order.customer.phone}
            </dd>
          </div>
          {order.fulfillment === 'DELIVERY' && order.address?.line && (
            <div className="sm:col-span-2">
              <dt className="text-stone-500">Alamat</dt>
              <dd className="text-stone-900">
                {[
                  order.address.line,
                  order.address.rt_rw,
                  order.address.kelurahan,
                  order.address.kecamatan,
                  order.address.city,
                  order.address.postal_code,
                ]
                  .filter(Boolean)
                  .join(', ')}
                {order.address.landmark ? ` (${order.address.landmark})` : ''}
              </dd>
            </div>
          )}
          {order.customer.note && (
            <div className="sm:col-span-2">
              <dt className="text-stone-500">Catatan</dt>
              <dd className="text-stone-900">{order.customer.note}</dd>
            </div>
          )}
        </dl>
      </section>

      {order.status_history.length > 0 && (
        <section className="rounded-2xl border border-stone-200 bg-white p-4">
          <h2 className="font-semibold text-stone-900">Riwayat status</h2>
          <ol className="mt-3 flex flex-col gap-3">
            {order.status_history.map((entry, index) => (
              <li key={`${entry.status}-${index}`} className="flex gap-3">
                <span
                  aria-hidden="true"
                  className="mt-1.5 size-2 shrink-0 rounded-full bg-brand-500"
                />
                <div>
                  <p className="text-sm font-medium text-stone-900">{statusLabel(entry.status)}</p>
                  <p className="text-xs text-stone-500">{formatDateTime(entry.at)}</p>
                  {entry.note && <p className="text-xs text-stone-500">{entry.note}</p>}
                </div>
              </li>
            ))}
          </ol>
        </section>
      )}

      {cancelError && <Alert tone="error">{cancelError}</Alert>}

      {onCancel && (
        <Button type="button" variant="danger" loading={cancelling} onClick={onCancel}>
          Batalkan pesanan
        </Button>
      )}
    </div>
  )
}
