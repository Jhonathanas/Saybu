import { useQuery } from '@tanstack/react-query'
import { useState } from 'react'
import { Card } from '@/components/ui/Card'
import { Alert, Spinner } from '@/components/ui/Feedback'
import { Input } from '@/components/ui/Input'
import { ApiError } from '@/lib/api'
import { formatDateTime } from '@/lib/format'
import { listUsers } from '@/features/auth/api'
import { usePageTitle } from '@/lib/seo'

export function AdminUsersPage() {
  usePageTitle('Pengguna · Admin')
  const [query, setQuery] = useState('')
  const [page, setPage] = useState(1)

  const { data, isLoading, isError, error } = useQuery({
    queryKey: ['admin', 'users', { page, q: query }],
    queryFn: () => listUsers({ page, limit: 20, q: query || undefined }),
  })

  const meta = data?.meta
  const totalPages = meta?.total_pages ?? 1

  return (
    <div className="flex flex-col gap-4">
      <h1 className="text-xl font-bold text-foreground">Pengguna</h1>

      <Card>
        <div className="max-w-sm">
          <Input
            label="Cari pengguna"
            placeholder="Nama, HP, atau email"
            value={query}
            onChange={(e) => {
              setQuery(e.target.value)
              setPage(1)
            }}
          />
        </div>
      </Card>

      {isLoading && <Spinner label="Memuat pengguna…" />}
      {isError && (
        <Alert tone="error">
          {error instanceof ApiError ? error.message : 'Gagal memuat pengguna.'}
        </Alert>
      )}

      {data && (
        <Card>
          <div className="overflow-x-auto">
            <table className="w-full text-left text-sm">
              <thead className="text-xs uppercase text-muted-foreground">
                <tr>
                  <th className="py-2 pr-3">Nama</th>
                  <th className="py-2 pr-3">HP</th>
                  <th className="py-2 pr-3">Email</th>
                  <th className="py-2 pr-3">Peran</th>
                  <th className="py-2">Terdaftar</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-border">
                {data.data.map((u) => (
                  <tr key={u.id}>
                    <td className="py-2 pr-3 font-medium text-foreground">{u.name}</td>
                    <td className="py-2 pr-3">{u.phone}</td>
                    <td className="py-2 pr-3">{u.email || '—'}</td>
                    <td className="py-2 pr-3">
                      <span
                        className={`rounded-full px-2 py-0.5 text-xs ${
                          u.role === 'admin'
                            ? 'bg-secondary text-foreground'
                            : 'bg-muted text-muted-foreground'
                        }`}
                      >
                        {u.role}
                      </span>
                    </td>
                    <td className="py-2 text-muted-foreground">
                      {u.created_at ? formatDateTime(u.created_at) : '—'}
                    </td>
                  </tr>
                ))}
                {data.data.length === 0 && (
                  <tr>
                    <td colSpan={5} className="py-6 text-center text-muted-foreground">
                      Tidak ada pengguna yang cocok.
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>

          <div className="mt-4 flex items-center justify-between text-sm text-muted-foreground">
            <span>
              Halaman {meta?.page ?? 1} dari {totalPages} · total {meta?.total ?? 0}
            </span>
            <div className="flex gap-2">
              <button
                type="button"
                className="tap-target rounded-lg px-3 ring-1 ring-border disabled:opacity-40"
                disabled={page <= 1}
                onClick={() => setPage((p) => Math.max(1, p - 1))}
              >
                Sebelumnya
              </button>
              <button
                type="button"
                className="tap-target rounded-lg px-3 ring-1 ring-border disabled:opacity-40"
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
