import type { ReactNode } from 'react'
import {
  Card as CardRoot,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from './shadcn/card'
import { cn } from '@/lib/utils'

interface CardProps {
  title?: string
  description?: string
  children: ReactNode
  className?: string
}

/** Kompatibel dengan API lama; bagian kartu memakai primitives shadcn/ui. */
export function Card({ title, description, children, className }: CardProps) {
  const hasHeading = Boolean(title || description)
  return (
    <CardRoot className={cn('shadow-sm', className)}>
      {hasHeading && (
        <CardHeader>
          {title && <CardTitle>{title}</CardTitle>}
          {description && <CardDescription>{description}</CardDescription>}
        </CardHeader>
      )}
      <CardContent>{children}</CardContent>
    </CardRoot>
  )
}

export { CardRoot, CardHeader, CardTitle, CardDescription, CardContent }
