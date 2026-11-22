import { Alert } from '@/components/ui/Feedback'
import { Button } from '@/components/ui/Button'
import { CopyButton } from '@/components/ui/CopyButton'
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

interface OrderDetailViewProps {
  order: Order
  onCancel?: () => void
  cancelling?: boolean
  cancelError?: string | null
}

/** OrderDetailView merender rincian pesanan (dipakai halaman sukses & lacak). */
export function OrderDetailView({
  order,
  onCancel,
  cancelling = false,
  cancelError,
}: OrderDetailViewProps) {
  return (
    <div className="flex flex-col gap-5">
      <section className="rounded-2xl border border-border bg-card p-4">
        <div className="flex flex-wrap items-center justify-between gap-2">
          <div>
            <p className="text-xs text-muted-foreground">Kode pesanan</p>
            <p className="font-mono text-lg font-bold text-foreground">{order.code}</p>
          </div>
          <div className="flex items-center gap-2">
            <StatusBadge status={order.status} />
            <CopyButton value={order.code} label="Salin kode" />
          </div>
        </div>
        <dl className="mt-4 grid gap-2 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-muted-foreground">Dibuat</dt>
            <dd className="text-foreground">{formatDateTime(order.created_at)}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">
              {isUnpaid(order.status) ? 'Batas pembayaran' : 'Status'}
            </dt>
            <dd className="text-foreground">
              {isUnpaid(order.status) ? formatDateTime(order.expires_at) : statusLabel(order.status)}
            </dd>
          </div>
        </dl>
        {order.cancel_reason && (
          <p className="mt-2 text-sm text-destructive">Alasan batal: {order.cancel_reason}</p>
        )}
      </section>

      <section className="rounded-2xl border border-border bg-card p-4">
        <h2 className="font-semibold text-foreground">Produk</h2>
        <ul className="mt-3 flex flex-col divide-y divide-border">
          {order.items.map((item) => (
            <li key={item.product_id} className="flex items-start justify-between gap-3 py-2">
              <div>
                <p className="font-medium text-foreground">{item.name}</p>
                <p className="text-sm text-muted-foreground">
                  {item.qty_display} × {formatIDR(item.unit_price)}
                </p>
              </div>
              <p className="font-medium text-foreground">{formatIDR(item.line_total)}</p>
            </li>
          ))}
        </ul>
        <dl className="mt-3 flex flex-col gap-1.5 border-t border-border pt-3 text-sm">
          <div className="flex justify-between">
            <dt className="text-muted-foreground">Subtotal</dt>
            <dd className="text-foreground">{formatIDR(order.subtotal)}</dd>
          </div>
          <div className="flex justify-between">
            <dt className="text-muted-foreground">Ongkir</dt>
            <dd className="text-foreground">
              {order.shipping_fee === 0 ? 'Gratis' : formatIDR(order.shipping_fee)}
            </dd>
          </div>
          <div className="flex justify-between text-base font-bold">
            <dt>Total</dt>
            <dd>{formatIDR(order.total)}</dd>
          </div>
        </dl>
      </section>

      <section className="rounded-2xl border border-border bg-card p-4">
        <h2 className="font-semibold text-foreground">Pengiriman</h2>
        <dl className="mt-3 grid gap-2 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-muted-foreground">Cara</dt>
            <dd className="text-foreground">{fulfillmentLabel(order.fulfillment)}</dd>
          </div>
          <div>
            <dt className="text-muted-foreground">Pembayaran</dt>
            <dd className="text-foreground">{paymentLabel(order.payment_method)}</dd>
          </div>
          {order.zone_name && (
            <div>
              <dt className="text-muted-foreground">Zona</dt>
              <dd className="text-foreground">{order.zone_name}</dd>
            </div>
          )}
          <div>
            <dt className="text-muted-foreground">Penerima</dt>
            <dd className="text-foreground">
              {order.customer.name} · {order.customer.phone}
            </dd>
          </div>
          {order.fulfillment === 'DELIVERY' && order.address?.line && (
            <div className="sm:col-span-2">
              <dt className="text-muted-foreground">Alamat</dt>
              <dd className="text-foreground">
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
              <dt className="text-muted-foreground">Catatan</dt>
              <dd className="text-foreground">{order.customer.note}</dd>
            </div>
          )}
        </dl>
      </section>

      {order.status_history.length > 0 && (
        <section className="rounded-2xl border border-border bg-card p-4">
          <h2 className="font-semibold text-foreground">Riwayat status</h2>
          <ol className="mt-3 flex flex-col gap-3">
            {order.status_history.map((entry, index) => (
              <li key={`${entry.status}-${index}`} className="flex gap-3">
                <span
                  aria-hidden="true"
                  className="mt-1.5 size-2 shrink-0 rounded-full bg-foreground"
                />
                <div>
                  <p className="text-sm font-medium text-foreground">{statusLabel(entry.status)}</p>
                  <p className="text-xs text-muted-foreground">{formatDateTime(entry.at)}</p>
                  {entry.note && <p className="text-xs text-muted-foreground">{entry.note}</p>}
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
