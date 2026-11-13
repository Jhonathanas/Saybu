import { Link } from 'react-router'
import { Button } from '@/components/ui/Button'
import { useCart } from '@/features/cart/store'
import { formatIDR, formatQty } from '@/lib/format'
import { ProductImage } from './ProductImage'
import type { Product } from './types'

export function ProductCard({ product }: { product: Product }) {
  const add = useCart((s) => s.add)
  const quickQty = product.min_qty > 0 ? product.min_qty : product.step_qty || 1

  return (
    <article className="flex flex-col overflow-hidden rounded-2xl border border-stone-200 bg-white shadow-sm">
      <Link to={`/produk/${product.slug}`} className="block">
        <ProductImage
          src={product.images[0]}
          alt={product.name}
          className="h-40 w-full sm:h-44"
        />
      </Link>

      <div className="flex flex-1 flex-col gap-2 p-4">
        <div>
          <Link to={`/produk/${product.slug}`} className="font-semibold text-stone-900 hover:text-brand-700">
            {product.name}
          </Link>
          {product.category && (
            <p className="text-xs text-stone-500">{product.category.name}</p>
          )}
        </div>

        <p className="text-sm font-semibold text-stone-900">
          {formatIDR(product.price)}
          <span className="text-xs font-normal text-stone-500"> / {product.unit}</span>
        </p>

        <p className="text-xs text-stone-500">
          {product.is_available ? `Stok ${formatQty(product.unit, product.stock)}` : 'Stok habis'}
        </p>

        <div className="mt-auto pt-2">
          <Button
            size="sm"
            className="w-full"
            disabled={!product.is_available}
            onClick={() => add(product, quickQty)}
            aria-label={`Tambah ${product.name} ke keranjang`}
          >
            {product.is_available ? 'Tambah' : 'Stok habis'}
          </Button>
        </div>
      </div>
    </article>
  )
}
