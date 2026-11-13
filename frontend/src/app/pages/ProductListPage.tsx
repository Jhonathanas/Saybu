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
    <div className="flex flex-col gap-5">
      <div>
        <h1 className="text-xl font-bold text-stone-900">Produk</h1>
        <p className="text-sm text-stone-500">Buah dan sayur segar, dikirim dari pasar pagi ini.</p>
      </div>

      <form
        className="flex flex-col gap-3 sm:flex-row sm:items-end"
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
        <div className="sm:w-48">
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

      <div className="flex flex-wrap gap-2">
        <button
          type="button"
          onClick={() => update({ category: '' })}
          className={`tap-target rounded-full border px-3 text-sm ${
            category === ''
              ? 'border-brand-600 bg-brand-50 text-brand-700'
              : 'border-stone-300 bg-white text-stone-600 hover:bg-stone-100'
          }`}
        >
          Semua
        </button>
        {(categories.data ?? []).map((c) => (
          <button
            key={c.id}
            type="button"
            onClick={() => update({ category: c.slug })}
            className={`tap-target rounded-full border px-3 text-sm ${
              category === c.slug
                ? 'border-brand-600 bg-brand-50 text-brand-700'
                : 'border-stone-300 bg-white text-stone-600 hover:bg-stone-100'
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

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
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
          <span className="text-sm text-stone-500">
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
