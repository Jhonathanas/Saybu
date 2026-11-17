import { Link } from 'react-router'

/** ServerErrorPage menampilkan pesan 500 (mis. saat backend tidak terjangkau). */
export function ServerErrorPage() {
  return (
    <div className="flex min-h-screen flex-col items-center justify-center gap-4 px-4 text-center">
      <p className="text-5xl font-bold text-red-500">500</p>
      <h1 className="text-xl font-semibold text-stone-900">Terjadi kesalahan server</h1>
      <p className="max-w-sm text-sm text-stone-500">
        Kami sedang mengalami gangguan. Silakan coba lagi beberapa saat lagi.
      </p>
      <Link
        to="/"
        className="tap-target inline-flex items-center rounded-lg bg-brand-600 px-4 text-sm font-medium text-white hover:bg-brand-700"
      >
        Kembali ke beranda
      </Link>
    </div>
  )
}
