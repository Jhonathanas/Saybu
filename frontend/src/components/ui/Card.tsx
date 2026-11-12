import type { ReactNode } from 'react'

export function Card({
  title,
  description,
  children,
}: {
  title?: string
  description?: string
  children: ReactNode
}) {
  return (
    <section className="rounded-2xl border border-stone-200 bg-white p-5 shadow-sm">
      {title && <h2 className="text-lg font-semibold text-stone-900">{title}</h2>}
      {description && <p className="mt-1 text-sm text-stone-500">{description}</p>}
      <div className={title || description ? 'mt-4' : ''}>{children}</div>
    </section>
  )
}
