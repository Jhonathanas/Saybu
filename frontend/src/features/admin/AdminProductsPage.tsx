import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useState } from 'react'
import { Button } from '@/components/ui/Button'
import { Card } from '@/components/ui/Card'
import { Alert, Spinner } from '@/components/ui/Feedback'
import { Input } from '@/components/ui/Input'
import { Select } from '@/components/ui/Select'
import { ApiError } from '@/lib/api'
import { formatIDR, formatQty } from '@/lib/format'
import {
  adminListProducts,
  createProduct,
  deleteProduct,
  listCategories,
  setStock,
  updateProduct,
  uploadImage,
} from '@/features/catalog/api'
import { UNITS, type AdminProduct, type ProductInput, type Unit } from '@/features/catalog/types'

interface FormState {
  name: string
  slug: string
  category_id: string
  description: string
  unit: Unit
  price: string
  stock_qty: string
  min_qty: string
  step_qty: string
  is_active: boolean
  images: string[]
}

const emptyForm: FormState = {
  name: '',
  slug: '',
  category_id: '',
  description: '',
  unit: 'kg',
  price: '',
  stock_qty: '',
  min_qty: '',
  step_qty: '',
  is_active: true,
  images: [],
}

function formFrom(p: AdminProduct): FormState {
  return {
    name: p.name,
    slug: p.slug,
    category_id: p.category_id,
    description: p.description,
    unit: p.unit,
    price: String(p.price),
    stock_qty: String(p.stock_qty),
    min_qty: String(p.min_qty),
    step_qty: String(p.step_qty),
    is_active: p.is_active,
    images: p.images,
  }
}

function toNumber(raw: string): number {
  const n = Number(raw.replace(/\./g, '').replace(',', '.'))
  return Number.isFinite(n) ? Math.round(n) : 0
}

export function AdminProductsPage() {
  const queryClient = useQueryClient()
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)
  const [editing, setEditing] = useState<AdminProduct | null>(null)
  const [creating, setCreating] = useState(false)
  const [stockTarget, setStockTarget] = useState<AdminProduct | null>(null)
  const [form, setForm] = useState<FormState>(emptyForm)
  const [formError, setFormError] = useState<string | null>(null)
  const [notice, setNotice] = useState<string | null>(null)

  const categories = useQuery({ queryKey: ['categories'], queryFn: listCategories })

  const products = useQuery({
    queryKey: ['admin', 'products', { page, q: query }],
    queryFn: () => adminListProducts({ page, limit: 20, q: query || undefined }),
  })

  const invalidate = () => {
    void queryClient.invalidateQueries({ queryKey: ['admin', 'products'] })
    void queryClient.invalidateQueries({ queryKey: ['products'] })
  }

  const save = useMutation({
    mutationFn: async () => {
      const base: ProductInput = {
        name: form.name.trim(),
        slug: form.slug.trim() || undefined,
        category_id: form.category_id || undefined,
        description: form.description.trim(),
        unit: form.unit,
        price: toNumber(form.price),
        stock_qty: toNumber(form.stock_qty),
        min_qty: toNumber(form.min_qty) || undefined,
        step_qty: toNumber(form.step_qty) || undefined,
        is_active: form.is_active,
        images: form.images,
      }
      if (editing) {
        return updateProduct(editing.id, base)
      }
      return createProduct(base)
    },
    onSuccess: () => {
      setNotice(editing ? 'Produk diperbarui.' : 'Produk dibuat.')
      setFormError(null)
      closeForm()
      invalidate()
    },
    onError: (err) => {
      setFormError(err instanceof ApiError ? err.message : 'Gagal menyimpan produk.')
    },
  })

  const remove = useMutation({
    mutationFn: (id: string) => deleteProduct(id),
    onSuccess: () => {
      setNotice('Produk dinonaktifkan.')
      invalidate()
    },
    onError: (err) => {
      setNotice(err instanceof ApiError ? err.message : 'Gagal menonaktifkan produk.')
    },
  })

  function openCreate() {
    setForm(emptyForm)
    setEditing(null)
    setCreating(true)
    setFormError(null)
  }

  function openEdit(p: AdminProduct) {
    setForm(formFrom(p))
    setEditing(p)
    setCreating(true)
    setFormError(null)
  }

  function closeForm() {
    setCreating(false)
    setEditing(null)
    setForm(emptyForm)
  }

  const meta = products.data?.meta
  const totalPages = meta?.total_pages ?? 1

  return (
    <div className="flex flex-col gap-4">
      <div className="flex items-center justify-between">
        <h1 className="text-xl font-bold text-stone-900">Produk</h1>
        <Button onClick={openCreate}>Tambah produk</Button>
      </div>

      {notice && (
        <Alert tone="success">
          <span className="flex items-center justify-between gap-3">
            {notice}
            <button type="button" className="underline" onClick={() => setNotice(null)}>
              Tutup
            </button>
          </span>
        </Alert>
      )}

      {creating && (
        <Card title={editing ? `Ubah: ${editing.name}` : 'Produk baru'}>
          <ProductForm
            form={form}
            setForm={setForm}
            categories={categories.data ?? []}
            error={formError}
            submitting={save.isPending}
            submitLabel={editing ? 'Simpan perubahan' : 'Buat produk'}
            onCancel={closeForm}
            onSubmit={() => save.mutate()}
          />
        </Card>
      )}

      {stockTarget && (
        <Card title={`Atur stok: ${stockTarget.name}`}>
          <StockForm
            product={stockTarget}
            onDone={(message) => {
              setNotice(message)
              setStockTarget(null)
              invalidate()
            }}
            onCancel={() => setStockTarget(null)}
          />
        </Card>
      )}

      <Card>
        <div className="max-w-sm">
          <Input
            label="Cari produk"
            placeholder="Nama produk"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value)
              setPage(1)
            }}
          />
        </div>
      </Card>

      {products.isLoading && <Spinner label="Memuat produk…" />}
      {products.isError && (
        <Alert tone="error">
          {products.error instanceof ApiError ? products.error.message : 'Gagal memuat produk.'}
        </Alert>
      )}

      {products.data && (
        <Card>
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="text-xs uppercase text-stone-500">
                <tr>
                  <th className="py-2 pr-3">Nama</th>
                  <th className="py-2 pr-3">Kategori</th>
                  <th className="py-2 pr-3">Harga</th>
                  <th className="py-2 pr-3">Stok</th>
                  <th className="py-2 pr-3">Status</th>
                  <th className="py-2">Aksi</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-stone-100">
                {products.data.data.map((p) => (
                  <tr key={p.id} className={p.is_active ? '' : 'opacity-60'}>
                    <td className="py-2 pr-3 font-medium text-stone-900">
                      {p.name}
                      <span className="block text-xs font-normal text-stone-400">/{p.slug}</span>
                    </td>
                    <td className="py-2 pr-3">{p.category?.name ?? '—'}</td>
                    <td className="py-2 pr-3 whitespace-nowrap">
                      {formatIDR(p.price)}
                      <span className="text-xs text-stone-400"> / {p.unit}</span>
                    </td>
                    <td className="py-2 pr-3 whitespace-nowrap">{formatQty(p.unit, p.stock_qty)}</td>
                    <td className="py-2 pr-3">
                      <span
                        className={`rounded-full px-2 py-0.5 text-xs ${
                          p.is_active
                            ? 'bg-brand-100 text-brand-800'
                            : 'bg-stone-100 text-stone-600'
                        }`}
                      >
                        {p.is_active ? 'aktif' : 'nonaktif'}
                      </span>
                    </td>
                    <td className="py-2">
                      <div className="flex flex-wrap gap-2">
                        <button
                          type="button"
                          className="tap-target rounded-lg px-2 ring-1 ring-stone-300 hover:bg-stone-100"
                          onClick={() => openEdit(p)}
                        >
                          Ubah
                        </button>
                        <button
                          type="button"
                          className="tap-target rounded-lg px-2 ring-1 ring-stone-300 hover:bg-stone-100"
                          onClick={() => {
                            setStockTarget(p)
                            setCreating(false)
                          }}
                        >
                          Atur stok
                        </button>
                        {p.is_active && (
                          <button
                            type="button"
                            className="tap-target rounded-lg px-2 text-red-600 ring-1 ring-red-200 hover:bg-red-50"
                            onClick={() => {
                              if (confirm(`Nonaktifkan ${p.name}?`)) remove.mutate(p.id)
                            }}
                          >
                            Nonaktifkan
                          </button>
                        )}
                      </div>
                    </td>
                  </tr>
                ))}
                {products.data.data.length === 0 && (
                  <tr>
                    <td colSpan={6} className="py-6 text-center text-stone-500">
                      Belum ada produk.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          <div className="mt-4 flex items-center justify-between text-sm text-stone-600">
            <span>
              Halaman {meta?.page ?? 1} dari {totalPages} · total {meta?.total ?? 0}
            </span>
            <div className="flex gap-2">
              <button
                type="button"
                className="tap-target rounded-lg px-3 ring-1 ring-stone-300 disabled:opacity-40"
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                Sebelumnya
              </button>
              <button
                type="button"
                className="tap-target rounded-lg px-3 ring-1 ring-stone-300 disabled:opacity-40"
                disabled={page >= totalPages}
                onClick={() => setPage((p) => p + 1)}
              >
                Berikutnya
              </button>
            </div>
          </div>
        </Card>
      )}
    </div>
  )
}

interface ProductFormProps {
  form: FormState
  setForm: (updater: (prev: FormState) => FormState) => void
  categories: { id: string; name: string }[]
  error: string | null
  submitting: boolean
  submitLabel: string
  onCancel: () => void
  onSubmit: () => void
}

function ProductForm({
  form,
  setForm,
  categories,
  error,
  submitting,
  submitLabel,
  onCancel,
  onSubmit,
}: ProductFormProps) {
  const [uploadError, setUploadError] = useState<string | null>(null)
  const [uploading, setUploading] = useState(false)
  const isWeight = form.unit === 'kg'

  const set = <K extends keyof FormState>(key: K, value: FormState[K]) =>
    setForm((prev) => ({ ...prev, [key]: value }))

  async function handleUpload(file: File) {
    setUploadError(null)
    setUploading(true)
    try {
      const { url } = await uploadImage(file)
      setForm((prev) => ({ ...prev, images: [...prev.images, url] }))
    } catch (err) {
      setUploadError(err instanceof ApiError ? err.message : 'Gagal mengunggah gambar.')
    } finally {
      setUploading(false)
    }
  }

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault()
        onSubmit()
      }}
    >
      {error && <Alert tone="error">{error}</Alert>}

      <div className="grid gap-4 sm:grid-cols-2">
        <Input
          label="Nama"
          required
          value={form.name}
          onChange={(e) => set('name', e.target.value)}
        />
        <Input
          label="Slug"
          hint="Kosongkan untuk dibuat otomatis dari nama."
          value={form.slug}
          onChange={(e) => set('slug', e.target.value)}
        />
        <Select
          label="Kategori"
          value={form.category_id}
          onChange={(e) => set('category_id', e.target.value)}
          options={[
            { value: '', label: 'Tanpa kategori' },
            ...categories.map((c) => ({ value: c.id, label: c.name })),
          ]}
        />
        <Select
          label="Satuan"
          value={form.unit}
          onChange={(e) => set('unit', e.target.value as Unit)}
          options={UNITS.map((u) => ({ value: u, label: u }))}
        />
        <Input
          label={isWeight ? 'Harga per kg (Rp)' : `Harga per ${form.unit} (Rp)`}
          inputMode="numeric"
          required
          value={form.price}
          onChange={(e) => set('price', e.target.value)}
        />
        <Input
          label={isWeight ? 'Stok (gram)' : `Stok (${form.unit})`}
          hint={isWeight ? '1000 gram = 1 kg.' : undefined}
          inputMode="numeric"
          required
          value={form.stock_qty}
          onChange={(e) => set('stock_qty', e.target.value)}
        />
        <Input
          label={isWeight ? 'Min. pembelian (gram)' : `Min. pembelian (${form.unit})`}
          inputMode="numeric"
          value={form.min_qty}
          onChange={(e) => set('min_qty', e.target.value)}
        />
        <Input
          label={isWeight ? 'Langkah (gram)' : `Langkah (${form.unit})`}
          inputMode="numeric"
          value={form.step_qty}
          onChange={(e) => set('step_qty', e.target.value)}
        />
      </div>

      <div className="flex flex-col gap-1.5">
        <label htmlFor="deskripsi" className="text-sm font-medium text-stone-700">
          Deskripsi
        </label>
        <textarea
          id="deskripsi"
          rows={3}
          value={form.description}
          onChange={(e) => set('description', e.target.value)}
          className="w-full rounded-lg border border-stone-300 bg-white px-3 py-2 text-sm text-stone-900"
        />
      </div>

      <label className="flex items-center gap-2 text-sm text-stone-700">
        <input
          type="checkbox"
          className="size-4"
          checked={form.is_active}
          onChange={(e) => set('is_active', e.target.checked)}
        />
        Aktif (tampil di toko)
      </label>

      <div className="flex flex-col gap-2">
        <span className="text-sm font-medium text-stone-700">Gambar</span>
        {form.images.length > 0 && (
          <ul className="flex flex-wrap gap-2">
            {form.images.map((url) => (
              <li key={url} className="relative">
                <img src={url} alt="" className="size-20 rounded-lg border border-stone-200 object-cover" />
                <button
                  type="button"
                  aria-label="Hapus gambar"
                  className="absolute -right-2 -top-2 grid size-6 place-items-center rounded-full bg-red-600 text-xs text-white"
                  onClick={() =>
                    setForm((prev) => ({
                      ...prev,
                      images: prev.images.filter((u) => u !== url),
                    }))
                  }
                >
                  ×
                </button>
              </li>
            ))}
          </ul>
        )}
        <input
          type="file"
          accept="image/png,image/jpeg,image/webp"
          disabled={uploading}
          onChange={(e) => {
            const file = e.target.files?.[0]
            if (file) void handleUpload(file)
            e.target.value = ''
          }}
          className="text-sm"
        />
        {uploading && <p className="text-xs text-stone-500">Mengunggah…</p>}
        {uploadError && <p className="text-xs text-red-600">{uploadError}</p>}
      </div>

      <div className="flex gap-2">
        <Button type="submit" loading={submitting}>
          {submitLabel}
        </Button>
        <Button type="button" variant="secondary" onClick={onCancel}>
          Batal
        </Button>
      </div>
    </form>
  )
}

interface StockFormProps {
  product: AdminProduct
  onDone: (message: string) => void
  onCancel: () => void
}

function StockForm({ product, onDone, onCancel }: StockFormProps) {
  const [mode, setMode] = useState<'set' | 'adjust'>('adjust')
  const [qty, setQty] = useState('')
  const [error, setError] = useState<string | null>(null)

  const mutate = useMutation({
    mutationFn: () => setStock(product.id, mode, toNumber(qty)),
    onSuccess: () => onDone(`Stok ${product.name} diperbarui.`),
    onError: (err) => {
      setError(err instanceof ApiError ? err.message : 'Gagal memperbarui stok.')
    },
  })

  const isWeight = product.unit === 'kg'

  return (
    <form
      className="flex flex-col gap-4"
      onSubmit={(e) => {
        e.preventDefault()
        mutate.mutate()
      }}
    >
      {error && <Alert tone="error">{error}</Alert>}

      <p className="text-sm text-stone-600">
        Stok saat ini: <strong>{formatQty(product.unit, product.stock_qty)}</strong>
      </p>

      <div className="grid gap-4 sm:grid-cols-2">
        <Select
          label="Mode"
          value={mode}
          onChange={(e) => setMode(e.target.value as 'set' | 'adjust')}
          options={[
            { value: 'adjust', label: 'Tambah/kurangi (boleh negatif)' },
            { value: 'set', label: 'Set nilai baru' },
          ]}
        />
        <Input
          label={isWeight ? 'Jumlah (gram)' : `Jumlah (${product.unit})`}
          hint={isWeight ? '1000 gram = 1 kg. Gunakan tanda minus untuk mengurangi.' : undefined}
          inputMode="numeric"
          required
          value={qty}
          onChange={(e) => setQty(e.target.value)}
        />
      </div>

      <div className="flex gap-2">
        <Button type="submit" loading={mutate.isPending}>
          Terapkan
        </Button>
        <Button type="button" variant="secondary" onClick={onCancel}>
          Batal
        </Button>
      </div>
    </form>
  )
}
