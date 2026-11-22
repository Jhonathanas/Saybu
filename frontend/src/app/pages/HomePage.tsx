import { useQuery } from '@tanstack/react-query'
import { Link } from 'react-router'
import { Card } from '@/components/ui/Card'
import { Alert, Spinner } from '@/components/ui/Feedback'
import { CategoryTile } from '@/features/catalog/CategoryTile'
import { ProductCard } from '@/features/catalog/ProductCard'
import { listCategories, listProducts } from '@/features/catalog/api'
import { useAuth } from '@/features/auth/AuthProvider'
import { usePageTitle } from '@/lib/seo'

const categoryImages: Record<string, string> = {
  buah: '/images/kategori-buah.webp',
  sayur: '/images/kategori-sayur.webp',
}

export function HomePage() {
  const { user } = useAuth()
  usePageTitle(undefined, 'Belanja buah & sayur segar di Saybu. Antar ke rumah, bayar transfer atau COD.')

  const categories = useQuery({ queryKey: ['categories'], queryFn: listCategories })
  const featured = useQuery({
    queryKey: ['products', { page: 1, limit: 6, sort: 'newest' }],
    queryFn: () => listProducts({ page: 1, limit: 6, sort: 'newest' }),
  })

  const quickLinks = [
    {
      id: 'all',
      name: 'Semua',
      slug: 'semua',
      sort: 0,
      to: '/produk',
      image: '/images/kategori-semua.webp',
    },
    ...(categories.data ?? []).map((c) => ({
      ...c,
      to: `/produk?category=${encodeURIComponent(c.slug)}`,
      image: categoryImages[c.slug] ?? '/images/kategori-semua.webp',
    })),
    {
      id: 'hemat',
      name: 'Hemat',
      slug: 'hemat',
      sort: 99,
      to: '/produk?sort=price_asc',
      image: '/images/kategori-hemat.webp',
    },
  ]

  return (
    <div className="flex flex-col gap-5">
      <section
        className="relative overflow-hidden rounded-3xl bg-foreground px-5 py-8 text-background sm:px-8 sm:py-12"
        style={{
          backgroundImage:
            'linear-gradient(90deg, color-mix(in oklab, var(--foreground) 94%, transparent) 0%, color-mix(in oklab, var(--foreground) 70%, transparent) 55%, color-mix(in oklab, var(--foreground) 25%, transparent) 100%), url(/images/hero-sayur.webp)',
          backgroundSize: 'cover',
          backgroundPosition: 'center',
        }}
      >
        <span className="inline-flex rounded-full bg-background px-3 py-1 text-[11px] font-bold uppercase tracking-wide text-foreground">
          Panen pagi ini
        </span>
        <h1 className="mt-3 max-w-md text-2xl font-extrabold leading-tight sm:text-4xl">
          Buah &amp; sayur segar, langsung ke dapurmu
        </h1>
        <p className="mt-2 max-w-md text-sm text-background/80">
          Pesan dari rumah, bayar transfer atau COD. Dikonfirmasi penjual lewat WhatsApp.
        </p>
        <div className="mt-5 flex flex-wrap gap-2.5">
          <Link
            to="/produk"
            className="tap-target inline-flex items-center rounded-xl bg-background px-5 text-sm font-bold text-foreground hover:bg-muted"
          >
            Lihat produk
          </Link>
          {!user && (
            <Link
              to="/daftar"
              className="tap-target inline-flex items-center rounded-xl border border-background/40 px-5 text-sm font-bold text-background hover:bg-background/10"
            >
              Daftar akun
            </Link>
          )}
        </div>
      </section>

      <section aria-labelledby="belanja-cepat">
        <h2 id="belanja-cepat" className="mb-2 text-base font-bold text-foreground">
          Belanja cepat
        </h2>
        <div className="grid grid-cols-4 gap-2 sm:grid-cols-6">
          {quickLinks.map((item) => (
            <CategoryTile
              key={item.id}
              category={{ id: item.id, name: item.name, slug: item.slug, sort: item.sort }}
              image={item.image}
              to={item.to}
            />
          ))}
        </div>
      </section>

      <section aria-labelledby="produk-terbaru" className="flex flex-col gap-3">
        <div className="flex items-baseline justify-between">
          <h2 id="produk-terbaru" className="text-lg font-bold text-foreground">
            Produk terbaru
          </h2>
          <Link to="/produk" className="text-sm font-semibold text-foreground hover:underline">
            Lihat semua
          </Link>
        </div>

        {featured.isLoading && <Spinner label="Memuat produk…" />}
        {featured.isError && <Alert tone="error">Gagal memuat produk.</Alert>}

        <div className="grid grid-cols-2 gap-3 sm:grid-cols-3 lg:grid-cols-3">
          {(featured.data?.data ?? []).map((p) => (
            <ProductCard key={p.id} product={p} />
          ))}
        </div>
      </section>

      {user && (
        <Card title={`Halo, ${user.name}`}>
          <p className="text-sm text-muted-foreground">
            Anda masuk sebagai <strong>{user.role}</strong>.{' '}
            <Link to="/akun" className="text-foreground hover:underline">
              Lihat profil
            </Link>
            {user.role === 'admin' && (
              <>
                {' '}
                atau buka{' '}
                <Link to="/admin" className="text-foreground hover:underline">
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
