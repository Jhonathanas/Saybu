import { useQuery } from '@tanstack/react-query'
import { useEffect, useState } from 'react'
import { useSearchParams } from 'react-router'
import { Button } from '@/components/ui/Button'
import { Alert, Spinner } from '@/components/ui/Feedback'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { ApiError } from '@/lib/api'
import { ProductCard } from '@/features/catalog/ProductCard'
import { listCategories, listProducts } from '@/features/catalog/api'
import type { ProductSort } from '@/features/catalog/types'
import { usePageTitle } from '@/lib/seo'

const sortOptions = [
  { value: 'newest', label: 'Terbaru' },
  { value: 'price_asc', label: 'Harga termurah' },
  { value: 'price_desc', label: 'Harga tertinggi' },
]

export function ProductListPage() {
  const [params, setParams] = useSearchParams()
  const category = params.get('category') ?? ''
  const q = params.get('q') ?? ''
  const sort = (params.get('sort') as ProductSort | null) ?? 'newest'
  const page = Number(params.get('page') ?? '1') || 1

  usePageTitle('Produk', 'Daftar buah dan sayur segar Saybu, dikirim dari pasar pagi ini.')

  const [search, setSearch] = useState(q)
  useEffect(() => setSearch(q), [q])

  const categories = useQuery({
    queryKey: ['categories'],
    queryFn: listCategories,
  })

  const products = useQuery({
    queryKey: ['products', { category, q, sort, page }],
    queryFn: () => listProducts({ category, q, sort, page, limit: 12 }),
  })

  function update(next: Record<string, string>) {
    const merged = new URLSearchParams(params)
    for (const [key, value] of Object.entries(next)) {
      if (value) merged.set(key, value)
      else merged.delete(key)
    }
    if (!('page' in next)) merged.delete('page')
    setParams(merged, { replace: true })
  }

  const meta = products.data?.meta
  const totalPages = meta?.total_pages ?? 1

  return (
    <div className="flex flex-col gap-4">
      <div>
        <h1 className="text-xl font-extrabold text-foreground">Produk</h1>
        <p className="text-sm text-muted-foreground">Buah dan sayur segar, dikirim dari pasar pagi ini.</p>
      </div>

      <form
        className="sticky top-16 z-10 flex flex-col gap-2 rounded-2xl border border-border bg-card/95 p-3 shadow-sm backdrop-blur sm:flex-row sm:items-end"
        onSubmit={(e) => {
          e.preventDefault()
          update({ q: search })
        }}
      >
        <div className="flex-1">
          <Input
            label="Cari produk"
            placeholder="Mis. bayam, tomat, apel"
            value={search}
            onChange={(e) => setSearch(e.target.value)}
          />
        </div>
        <div className="sm:w-44">
          <Select
            label="Urutkan"
            value={sort}
            options={sortOptions}
            onChange={(e) => update({ sort: e.target.value })}
          />
        </div>
        <Button type="submit" className="sm:mb-0">
          Cari
        </Button>
      </form>

      <div className="-mx-4 flex gap-2 overflow-x-auto px-4 pb-1 sm:mx-0 sm:px-0">
        <button
          type="button"
          onClick={() => update({ category: '' })}
          className={`tap-target shrink-0 rounded-full border px-4 text-sm font-medium ${
            category === ''
              ? 'border-foreground bg-muted text-foreground'
              : 'border-border bg-card text-muted-foreground hover:bg-muted'
          }`}
        >
          Semua
        </button>
        {(categories.data ?? []).map((c) => (
          <button
            key={c.id}
            type="button"
            onClick={() => update({ category: c.slug })}
            className={`tap-target shrink-0 rounded-full border px-4 text-sm font-medium ${
              category === c.slug
                ? 'border-foreground bg-muted text-foreground'
                : 'border-border bg-card text-muted-foreground hover:bg-muted'
            }`}
          >
            {c.name}
          </button>
        ))}
      </div>

      {products.isLoading && <Spinner label="Memuat produk…" />}

      {products.isError && (
        <Alert tone="error">
          {(products.error as ApiError).message ?? 'Gagal memuat produk.'}
        </Alert>
      )}

      {products.data && products.data.data.length === 0 && (
        <Alert tone="info">Tidak ada produk yang cocok. Coba kata kunci lain.</Alert>
      )}

      <div className="grid grid-cols-2 gap-3 sm:grid-cols-3">
        {(products.data?.data ?? []).map((p) => (
          <ProductCard key={p.id} product={p} />
        ))}
      </div>

      {totalPages > 1 && (
        <div className="flex items-center justify-between">
          <Button
            variant="secondary"
            size="sm"
            disabled={page <= 1}
            onClick={() => update({ page: String(page - 1) })}
          >
            Sebelumnya
          </Button>
          <span className="text-sm text-muted-foreground">
            Halaman {page} dari {totalPages}
          </span>
          <Button
            variant="secondary"
            size="sm"
            disabled={page >= totalPages}
            onClick={() => update({ page: String(page + 1) })}
          >
            Berikutnya
          </Button>
        </div>
      )}
    </div>
  )
}
