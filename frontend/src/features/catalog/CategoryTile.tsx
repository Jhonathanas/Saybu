import { Link } from 'react-router'
import { ProductImage } from './ProductImage'
import type { Category } from './types'

/** Kotak kategori untuk grid beranda (4 kolom di mobile). */
export function CategoryTile({
  category,
  image,
  to,
}: {
  category: Category
  image?: string
  to?: string
}) {
  return (
    <Link
      to={to ?? `/produk?category=${encodeURIComponent(category.slug)}`}
      className="tap-target flex min-h-0 flex-col items-center gap-1.5 rounded-2xl border border-border bg-card p-2 text-center shadow-sm transition hover:border-foreground/30 hover:bg-muted"
    >
      <ProductImage
        src={image}
        alt={category.name}
        className="size-14 rounded-xl sm:size-16"
      />
      <span className="line-clamp-1 text-xs font-semibold text-foreground">{category.name}</span>
    </Link>
  )
}
