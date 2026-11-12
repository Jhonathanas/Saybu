import { Card } from '@/components/ui/Card'

export function AdminDashboardPage() {
  return (
    <div className="flex flex-col gap-6">
      <h1 className="text-xl font-bold text-stone-900">Ringkasan</h1>

      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
        <Card title="Pesanan hari ini">
          <p className="text-2xl font-bold text-stone-900">—</p>
          <p className="mt-1 text-xs text-stone-500">Tersedia pada milestone M5.</p>
        </Card>
        <Card title="Menunggu verifikasi">
          <p className="text-2xl font-bold text-stone-900">—</p>
          <p className="mt-1 text-xs text-stone-500">Tersedia pada milestone M4.</p>
        </Card>
        <Card title="Produk aktif">
          <p className="text-2xl font-bold text-stone-900">—</p>
          <p className="mt-1 text-xs text-stone-500">Tersedia pada milestone M2.</p>
        </Card>
      </div>

      <Card title="Selamat datang" description="Panel admin Saybu">
        <p className="text-sm text-stone-600">
          Gunakan menu di samping untuk mengelola pengguna. Menu produk, pesanan, dan
          pembayaran akan muncul seiring bertambahnya milestone.
        </p>
      </Card>
    </div>
  )
}
