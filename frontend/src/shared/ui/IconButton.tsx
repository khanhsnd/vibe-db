import type { ButtonHTMLAttributes, ReactNode } from 'react'

import { cn } from '@/lib/cn'

interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  label: string
  icon: ReactNode
}

export function IconButton({ label, icon, className, ...rest }: IconButtonProps) {
  return (
    <button
      type="button"
      aria-label={label}
      title={label}
      className={cn(
        'inline-grid h-6 w-6 shrink-0 place-items-center rounded-md',
        'text-muted transition-colors hover:bg-hover hover:text-fg',
        'disabled:pointer-events-none disabled:opacity-45',
        className,
      )}
      {...rest}
    >
      {icon}
    </button>
  )
}
