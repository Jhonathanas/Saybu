import { Card } from '@/components/ui/Card'
import { useAuth } from '@/features/auth/AuthProvider'
import { formatDateTime } from '@/lib/format'

export function AccountPage() {
  const { user } = useAuth()
  if (!user) return null

  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-xl font-bold text-stone-900">Akun Saya</h1>

      <Card title="Profil">
        <dl className="grid gap-3 text-sm sm:grid-cols-2">
          <div>
            <dt className="text-stone-500">Nama</dt>
            <dd className="font-medium text-stone-900">{user.name}</dd>
          </div>
          <div>
            <dt className="text-stone-500">Nomor HP</dt>
            <dd className="font-medium text-stone-900">{user.phone}</dd>
          </div>
          <div>
            <dt className="text-stone-500">Email</dt>
            <dd className="font-medium text-stone-900">{user.email || '—'}</dd>
          </div>
          <div>
            <dt className="text-stone-500">Peran</dt>
            <dd className="font-medium text-stone-900">{user.role}</dd>
          </div>
          {user.created_at && (
            <div>
              <dt className="text-stone-500">Terdaftar</dt>
              <dd className="font-medium text-stone-900">{formatDateTime(user.created_at)}</dd>
            </div>
          )}
        </dl>
      </Card>

      <Card title="Riwayat pesanan">
        <p className="text-sm text-stone-600">
          Belum ada pesanan. Fitur riwayat pesanan ditambahkan pada milestone M5.
        </p>
      </Card>
    </div>
  )
}
