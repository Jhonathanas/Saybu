import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
import { Card } from '@/components/ui/Card'
import { Alert, Spinner } from '@/components/ui/Feedback'
import { ProductCard } from '@/features/catalog/ProductCard'
import { listCategories, listProducts } from '@/features/catalog/api'
import { useAuth } from '@/features/auth/AuthProvider'
import { usePageTitle } from '@/lib/seo'

export function HomePage() {
  const { user } = useAuth()
  usePageTitle(undefined, 'Belanja buah & sayur segar di Saybu. Antar ke rumah, bayar transfer atau COD.')

  const categories = useQuery({ queryKey: ['categories'], queryFn: listCategories })
  const featured = useQuery({
    queryKey: ['products', { page: 1, limit: 6, sort: 'newest' }],
    queryFn: () => listProducts({ page: 1, limit: 6, sort: 'newest' }),
  })

  return (
    <div className="flex flex-col gap-6">
      <section className="rounded-2xl bg-brand-600 px-6 py-10 text-white">
        <h1 className="text-2xl font-bold sm:text-3xl">Buah & sayur segar, antar ke rumah</h1>
        <p className="mt-2 max-w-xl text-sm text-brand-50">
          Pesan dari rumah, bayar dengan transfer bank atau COD. Dikonfirmasi langsung oleh
          penjual lewat WhatsApp.
        </p>
        <div className="mt-5 flex flex-wrap gap-3">
          <Link
            to="/produk"
            className="tap-target inline-flex items-center rounded-lg bg-white px-4 text-sm font-semibold text-brand-700 hover:bg-brand-50"
          >
            Lihat produk
          </Link>
          {!user && (
            <Link
              to="/daftar"
              className="tap-target inline-flex items-center rounded-lg bg-brand-700 px-4 text-sm font-semibold text-white hover:bg-brand-800"
            >
              Daftar akun
            </Link>
          )}
        </div>
      </section>

      {categories.data && categories.data.length > 0 && (
        <div className="flex flex-wrap gap-2">
          {categories.data.map((c) => (
            <Link
              key={c.id}
              to={`/produk?category=${c.slug}`}
              className="tap-target inline-flex items-center rounded-full border border-stone-300 bg-white px-4 text-sm text-stone-700 hover:bg-stone-100"
            >
              {c.name}
            </Link>
          ))}
        </div>
      )}

      <section className="flex flex-col gap-3">
        <div className="flex items-baseline justify-between">
          <h2 className="text-lg font-semibold text-stone-900">Produk terbaru</h2>
          <Link to="/produk" className="text-sm text-brand-700 hover:underline">
            Lihat semua
          </Link>
        </div>

        {featured.isLoading && <Spinner label="Memuat produk…" />}
        {featured.isError && <Alert tone="error">Gagal memuat produk.</Alert>}

        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {(featured.data?.data ?? []).map((p) => (
            <ProductCard key={p.id} product={p} />
          ))}
        </div>
      </section>

      {user && (
        <Card title={`Halo, ${user.name}`}>
          <p className="text-sm text-stone-600">
            Anda masuk sebagai <strong>{user.role}</strong>.{' '}
            <Link to="/akun" className="text-brand-700 hover:underline">
              Lihat profil
            </Link>
            {user.role === 'admin' && (
              <>
                {' '}
                atau buka{' '}
                <Link to="/admin" className="text-brand-700 hover:underline">
                  panel admin
                </Link>
                .
              </>
            )}
          </p>
        </Card>
      )}
    </div>
  )
}
