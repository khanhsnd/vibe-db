import type { ReactNode } from 'react'
import * as RadixDialog from '@radix-ui/react-dialog'
import { X } from 'lucide-react'

import { cn } from '@/lib/cn'

import { IconButton } from './IconButton'

/**
 * The tile behind the header icon, which is the one place a dialog says what kind it is.
 * `plain` is for an icon that carries its own colour — the engine chip is one.
 */
const TONES = {
  accent: 'bg-accent-subtle text-accent',
  danger: 'bg-danger-subtle text-danger',
  plain: '',
} as const

interface DialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
  title: string
  description?: string
  /** Drawn in a tinted tile left of the title. */
  icon?: ReactNode
  tone?: keyof typeof TONES
  children: ReactNode
  footer?: ReactNode
  width?: number
}

export function Dialog({
  open,
  onOpenChange,
  title,
  description,
  icon,
  tone = 'accent',
  children,
  footer,
  width = 460,
}: DialogProps) {
  return (
    <RadixDialog.Root open={open} onOpenChange={onOpenChange}>
      <RadixDialog.Portal>
        <RadixDialog.Overlay className="fixed inset-0 z-50 bg-black/50" />
        <RadixDialog.Content
          style={{ width }}
          className="fixed top-1/2 left-1/2 z-50 max-h-[85vh] -translate-x-1/2 -translate-y-1/2 overflow-hidden rounded-lg border border-line-strong bg-elevated shadow-[var(--shadow-overlay)]"
        >
          <div className="flex items-center gap-3 border-b border-line bg-bar px-4 py-3">
            {icon ? (
              <span
                className={cn('grid h-7 w-7 shrink-0 place-items-center rounded-md', TONES[tone])}
              >
                {icon}
              </span>
            ) : null}

            <div className="min-w-0 flex-1">
              <RadixDialog.Title className="text-base font-semibold text-fg">
                {title}
              </RadixDialog.Title>
              {description ? (
                <RadixDialog.Description className="mt-0.5 text-xs text-muted">
                  {description}
                </RadixDialog.Description>
              ) : null}
            </div>
            <RadixDialog.Close asChild>
              <IconButton label="Close" icon={<X size={14} />} />
            </RadixDialog.Close>
          </div>

          <div className="scroll-thin max-h-[60vh] overflow-y-auto px-4 py-3.5">{children}</div>

          {footer ? (
            <div className="flex items-center justify-end gap-2 border-t border-line bg-bar px-4 py-2.5">
              {footer}
            </div>
          ) : null}
        </RadixDialog.Content>
      </RadixDialog.Portal>
    </RadixDialog.Root>
  )
}
