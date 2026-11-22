import type { ButtonHTMLAttributes } from 'react'
import { LoaderCircle } from 'lucide-react'
import { Button as ButtonPrimitive, buttonVariants } from './shadcn/button'
import { cn } from '@/lib/utils'

type Variant = 'primary' | 'secondary' | 'ghost' | 'danger' | 'outline' | 'link'
type Size = 'md' | 'sm' | 'lg' | 'icon'

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  size?: Size
  loading?: boolean
}

/** Nama lama dipetakan ke variant shadcn/ui. */
const variants: Record<Variant, 'default' | 'outline' | 'ghost' | 'destructive' | 'link'> = {
  primary: 'default',
  secondary: 'outline',
  ghost: 'ghost',
  danger: 'destructive',
  outline: 'outline',
  link: 'link',
}

const sizes: Record<Size, 'default' | 'sm' | 'lg' | 'icon'> = {
  md: 'default',
  sm: 'sm',
  lg: 'lg',
  icon: 'icon',
}

export function Button({
  variant = 'primary',
  size = 'md',
  loading = false,
  className,
  disabled,
  children,
  ...rest
}: ButtonProps) {
  return (
    <ButtonPrimitive
      {...rest}
      disabled={disabled || loading}
      data-slot="button"
      className={cn('tap-target', buttonVariants({ variant: variants[variant], size: sizes[size] }), className)}
    >
      {loading && <LoaderCircle aria-hidden="true" className="size-4 animate-spin" />}
      {children}
    </ButtonPrimitive>
  )
}

export { buttonVariants }
