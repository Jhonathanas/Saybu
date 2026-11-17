import { Link, useNavigate } from 'react-router'
import { Button } from '@/components/ui/Button'
import { Alert } from '@/components/ui/Feedback'
import { QtyPicker } from '@/features/catalog/QtyPicker'
import { ProductImage } from '@/features/catalog/ProductImage'
import { lineTotal, useCart } from '@/features/cart/store'
import { formatIDR } from '@/lib/format'
import { usePageTitle } from '@/lib/seo'

export function CartPage() {
  usePageTitle('Keranjang')
  const lines = useCart((s) => s.lines)
  const setQty = useCart((s) => s.setQty)
  const remove = useCart((s) => s.remove)
  const clear = useCart((s) => s.clear)
  const navigate = useNavigate()

  const subtotal = lines.reduce((sum, line) => sum + lineTotal(line), 0)

  if (lines.length === 0) {
    return (
      <div className="flex flex-col gap-4">
        <h1 className="text-xl font-bold text-stone-900">Keranjang</h1>
        <Alert tone="info">
          Keranjang masih kosong.{' '}
          <Link to="/produk" className="font-medium underline">
            Mulai belanja
          </Link>
          .
        </Alert>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-5">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-stone-900">Keranjang</h1>
        <Button variant="ghost" size="sm" onClick={clear}>
          Kosongkan
        </Button>
      </div>

      <ul className="flex flex-col gap-3">
        {lines.map((line) => (
          <li
            key={line.productId}
            className="flex flex-col gap-3 rounded-2xl border border-stone-200 bg-white p-3 sm:flex-row sm:items-center"
          >
            <ProductImage
              src={line.image}
              alt={line.name}
              className="h-20 w-full rounded-xl sm:size-20"
            />

            <div className="flex-1">
              <Link
                to={`/produk/${line.slug}`}
                className="font-medium text-stone-900 hover:text-brand-700"
              >
                {line.name}
              </Link>
              <p className="text-sm text-stone-500">
                {formatIDR(line.price)} / {line.unit}
              </p>
            </div>

            <QtyPicker
              unit={line.unit}
              value={line.qty}
              min={line.minQty}
              step={line.stepQty}
              max={line.stock}
              onChange={(value) => setQty(line.productId, value)}
            />

            <div className="flex items-center justify-between gap-3 sm:flex-col sm:items-end">
              <p className="font-semibold text-stone-900">{formatIDR(lineTotal(line))}</p>
              <button
                type="button"
                onClick={() => remove(line.productId)}
                className="text-sm text-red-600 hover:underline"
              >
                Hapus
              </button>
            </div>
          </li>
        ))}
      </ul>

      <div className="flex flex-col gap-3 rounded-2xl border border-stone-200 bg-white p-4">
        <div className="flex items-center justify-between text-lg font-bold text-stone-900">
          <span>Subtotal</span>
          <span>{formatIDR(subtotal)}</span>
        </div>
        <p className="text-xs text-stone-500">
          Ongkir dan metode pengambilan dihitung di halaman checkout.
        </p>
        <Button onClick={() => navigate('/checkout')}>Checkout</Button>
      </div>
    </div>
  )
}
