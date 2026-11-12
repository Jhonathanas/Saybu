import type { ReactNode } from 'react'

export function Spinner({ label = 'Memuat…' }: { label?: string }) {
  return (
    <div role="status" className="flex items-center justify-center gap-3 py-10 text-stone-500">
      <span
        aria-hidden="true"
        className="size-5 animate-spin rounded-full border-2 border-stone-300 border-t-brand-600"
      />
      <span className="text-sm">{label}</span>
    </div>
  )
}

export function PageLoader() {
  return (
    <div className="flex min-h-screen items-center justify-center">
      <Spinner />
    </div>
  )
}

type AlertTone = 'error' | 'info' | 'success'

const tones: Record<AlertTone, string> = {
  error: 'border-red-200 bg-red-50 text-red-800',
  info: 'border-sky-200 bg-sky-50 text-sky-800',
  success: 'border-brand-200 bg-brand-50 text-brand-800',
}

export function Alert({
  tone = 'info',
  children,
}: {
  tone?: AlertTone
  children: ReactNode
}) {
  return (
    <div role="alert" className={`rounded-lg border px-3 py-2 text-sm ${tones[tone]}`}>
      {children}
    </div>
  )
}
