import { Link } from 'react-router'
import { Button } from '@/components/ui/Button'
import { useCart } from '@/features/cart/store'
import { formatQty } from '@/lib/format'
import { PriceTag } from './PriceTag'
import { ProductImage } from './ProductImage'
import type { Product } from './types'

function PlusIcon() {
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2.2" className="size-4" aria-hidden="true">
      <path d="M12 5v14M5 12h14" strokeLinecap="round" />
    </svg>
  )
}

export function ProductCard({ product }: { product: Product }) {
  const add = useCart((s) => s.add)
  const quickQty = product.min_qty > 0 ? product.min_qty : product.step_qty || 1

  return (
    <article className="group flex flex-col overflow-hidden rounded-2xl border border-border bg-card shadow-sm transition hover:-translate-y-0.5 hover:shadow-md">
      <Link to={`/produk/${product.slug}`} className="relative block">
        <ProductImage
          src={product.images[0]}
          alt={product.name}
          className="h-36 w-full sm:h-44"
        />
        {product.category && (
          <span className="absolute left-2 top-2 rounded-full bg-card/90 px-2.5 py-1 text-[11px] font-semibold text-foreground shadow-sm backdrop-blur">
            {product.category.name}
          </span>
        )}
        {!product.is_available && (
          <span className="absolute right-2 top-2 rounded-full bg-foreground/80 px-2.5 py-1 text-[11px] font-semibold text-background">
            Habis
          </span>
        )}
      </Link>

      <div className="flex flex-1 flex-col gap-2 p-3 sm:p-4">
        <Link
          to={`/produk/${product.slug}`}
          className="line-clamp-2 text-sm font-semibold leading-snug text-foreground hover:text-foreground"
        >
          {product.name}
        </Link>

        <PriceTag price={product.price} unit={product.unit} />

        <p className="text-xs text-muted-foreground">
          {product.is_available ? `Stok ${formatQty(product.unit, product.stock)}` : 'Stok habis'}
        </p>

        <div className="mt-auto pt-1">
          <Button
            size="sm"
            className="w-full"
            disabled={!product.is_available}
            onClick={() => add(product, quickQty)}
            aria-label={`Tambah ${product.name} ke keranjang`}
          >
            <PlusIcon />
            {product.is_available ? 'Tambah' : 'Stok habis'}
          </Button>
        </div>
      </div>
    </article>
  )
}
