import type { ReactNode } from 'react'
import { CircleAlert, CircleCheck, Info, LoaderCircle } from 'lucide-react'
import { Alert as AlertPrimitive, AlertTitle, AlertDescription } from './shadcn/alert'
import { cn } from '@/lib/utils'

export function Spinner({ label = 'Memuat…' }: { label?: string }) {
  return (
    <div role="status" className="flex items-center justify-center gap-3 py-10 text-muted-foreground">
      <LoaderCircle aria-hidden="true" className="size-5 animate-spin" />
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

const icons = {
  error: CircleAlert,
  info: Info,
  success: CircleCheck,
}

const toneClass: Record<AlertTone, string> = {
  error: '',
  info: 'border-border bg-card text-card-foreground',
  success: 'border-foreground/20 bg-card text-card-foreground',
}

/** API lama (`tone`) dipetakan ke komponen Alert shadcn/ui. */
export function Alert({ tone = 'info', children }: { tone?: AlertTone; children: ReactNode }) {
  const Icon = icons[tone]
  return (
    <AlertPrimitive
      variant={tone === 'error' ? 'destructive' : 'default'}
      className={cn('gap-2', toneClass[tone])}
    >
      <Icon aria-hidden="true" className="size-4" />
      <AlertDescription className="text-current">{children}</AlertDescription>
    </AlertPrimitive>
  )
}

export { AlertPrimitive, AlertTitle, AlertDescription }
