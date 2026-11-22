import { Link } from 'react-router'
import { usePageTitle } from '@/lib/seo'

export function NotFoundPage() {
  usePageTitle('Halaman tidak ditemukan')
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 px-4 text-center">
      <p className="text-5xl font-bold text-foreground">404</p>
      <h1 className="text-xl font-semibold text-foreground">Halaman tidak ditemukan</h1>
      <p className="max-w-sm text-sm text-muted-foreground">
        Alamat yang Anda buka tidak tersedia atau sudah dipindahkan.
      </p>
      <Link
        to="/"
        className="tap-target inline-flex items-center rounded-lg bg-primary px-4 text-sm font-medium text-background hover:bg-primary/90"
      >
        Kembali ke beranda
      </Link>
    </div>
  )
}
