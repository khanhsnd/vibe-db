import type { ButtonHTMLAttributes, ReactNode } from 'react'

import { cn } from '@/lib/cn'

type Variant = 'primary' | 'default' | 'subtle' | 'danger'

const VARIANTS: Record<Variant, string> = {
  primary: 'bg-accent text-on-accent hover:bg-accent-hover border-transparent',
  default: 'bg-elevated text-fg border-line-strong hover:bg-hover',
  subtle: 'bg-transparent text-muted border-transparent hover:bg-hover hover:text-fg',
  danger: 'bg-transparent text-danger border-line-strong hover:bg-danger-subtle',
}

interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: Variant
  icon?: ReactNode
}

export function Button({ variant = 'default', icon, className, children, ...rest }: ButtonProps) {
  return (
    <button
      type="button"
      className={cn(
        'inline-flex h-7 shrink-0 items-center gap-1.5 rounded-md border px-2.5',
        'text-[12.5px] font-medium whitespace-nowrap transition-colors',
        'disabled:pointer-events-none disabled:opacity-45',
        VARIANTS[variant],
        className,
      )}
      {...rest}
    >
      {icon}
      {children}
    </button>
  )
}
