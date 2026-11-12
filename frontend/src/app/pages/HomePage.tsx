import { Link } from 'react-router'
import { Card } from '@/components/ui/Card'
import { useAuth } from '@/features/auth/AuthProvider'

export function HomePage() {
  const { user } = useAuth()

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
            to="/daftar"
            className="tap-target inline-flex items-center rounded-lg bg-white px-4 text-sm font-semibold text-brand-700 hover:bg-brand-50"
          >
            Daftar akun
          </Link>
          <Link
            to="/masuk"
            className="tap-target inline-flex items-center rounded-lg bg-brand-700 px-4 text-sm font-semibold text-white hover:bg-brand-800"
          >
            Masuk
          </Link>
        </div>
      </section>

      <Card
        title="Katalog segera hadir"
        description="Milestone M2 menambahkan daftar produk, pencarian, dan keranjang belanja."
      >
        <p className="text-sm text-stone-600">
          Saat ini fondasi akun sudah berjalan: daftar, masuk, dan sesi yang aman.
        </p>
      </Card>

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
