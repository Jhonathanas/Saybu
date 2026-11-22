import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Link, useParams } from 'react-router'
import { Button } from '@/components/ui/Button'
import { Alert, Spinner } from '@/components/ui/Feedback'
import { useCart } from '@/features/cart/store'
import { QtyPicker } from '@/features/catalog/QtyPicker'
import { ProductImage } from '@/features/catalog/ProductImage'
import { getProduct } from '@/features/catalog/api'
import { formatIDR, formatQty } from '@/lib/format'
import { ApiError } from '@/lib/api'
import { usePageTitle } from '@/lib/seo'

export function ProductDetailPage() {
  const { slug = '' } = useParams()
  const add = useCart((s) => s.add)
  const line = useCart((s) => s.lines.find((l) => l.slug === slug))
  const [qty, setQty] = useState<number | null>(null)
  const [activeImage, setActiveImage] = useState(0)

  const { data: product, isLoading, isError, error } = useQuery({
    queryKey: ['product', slug],
    queryFn: () => getProduct(slug),
    enabled: slug !== '',
  })

  usePageTitle(
    product?.name,
    product
      ? `${product.name} ${formatIDR(product.price)} / ${product.unit}. Pesan online di Saybu.`
      : undefined,
  )

  if (isLoading) return <Spinner label="Memuat produk…" />
  if (isError || !product) {
    return (
      <Alert tone="error">
        {(error as ApiError)?.message ?? 'Produk tidak ditemukan.'}
      </Alert>
    )
  }

  const step = product.step_qty > 0 ? product.step_qty : 1
  const min = product.min_qty > 0 ? product.min_qty : step
  const selectedQty = qty ?? min

  return (
    <div className="flex flex-col gap-6">
      <Link to="/produk" className="text-sm text-foreground hover:underline">
        ← Kembali ke daftar produk
      </Link>

      <div className="grid gap-6 sm:grid-cols-2">
        <div className="flex flex-col gap-3">
          <ProductImage
            src={product.images[activeImage]}
            alt={product.name}
            className="h-64 w-full rounded-2xl border border-border sm:h-80"
          />
          {product.images.length > 1 && (
            <div className="flex gap-2">
              {product.images.map((url, i) => (
                <button
                  key={url}
                  type="button"
                  onClick={() => setActiveImage(i)}
                  aria-label={`Lihat foto ${i + 1}`}
                  className={`overflow-hidden rounded-lg border ${
                    i === activeImage ? 'border-foreground' : 'border-border'
                  }`}
                >
                  <ProductImage src={url} alt={`${product.name} ${i + 1}`} className="size-16" />
                </button>
              ))}
            </div>
          )}
        </div>

        <div className="flex flex-col gap-4">
          {product.category && (
            <span className="w-fit rounded-full bg-muted px-3 py-1 text-xs font-medium text-foreground">
              {product.category.name}
            </span>
          )}
          <h1 className="text-2xl font-bold text-foreground">{product.name}</h1>
          <p className="text-2xl font-bold text-foreground">
            {formatIDR(product.price)}
            <span className="text-sm font-normal text-muted-foreground"> / {product.unit}</span>
          </p>
          <p className="text-sm text-muted-foreground">{product.description || 'Belum ada deskripsi.'}</p>

          <p className="text-sm text-muted-foreground">
            {product.is_available
              ? `Stok tersedia: ${formatQty(product.unit, product.stock)}`
              : 'Stok sedang habis.'}
          </p>

          <div className="flex flex-col gap-3">
            <QtyPicker
              unit={product.unit}
              value={selectedQty}
              min={min}
              step={step}
              max={product.stock}
              disabled={!product.is_available}
              onChange={setQty}
            />
            <Button
              disabled={!product.is_available}
              onClick={() => add(product, selectedQty)}
              className="w-full sm:w-64"
            >
              Tambah ke keranjang
            </Button>
            {line && (
              <p className="text-sm text-muted-foreground">
                Di keranjang: {formatQty(product.unit, line.qty)}.{' '}
                <Link to="/keranjang" className="text-foreground hover:underline">
                  Lihat keranjang
                </Link>
              </p>
            )}
          </div>
        </div>
      </div>
    </div>
  )
}
