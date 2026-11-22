import { formatIDR } from '@/lib/format'

/** Harga per satuan dengan aksen warna yang konsisten di kartu produk. */
export function PriceTag({
  price,
  unit,
  size = 'md',
}: {
  price: number
  unit: string
  size?: 'md' | 'lg'
}) {
  return (
    <p className="flex flex-wrap items-baseline gap-x-1">
      <span
        className={
          size === 'lg'
            ? 'text-2xl font-extrabold text-foreground sm:text-3xl'
            : 'text-base font-extrabold text-foreground'
        }
      >
        {formatIDR(price)}
      </span>
      <span className="text-xs font-medium text-muted-foreground">/ {unit}</span>
    </p>
  )
}
