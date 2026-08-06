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
  /** What the body is padded with, for a dialog that lays out its own — Options has a side rail. */
  bodyClassName?: string
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
  bodyClassName = 'px-4 py-3.5',
}: DialogProps) {
  return (
    <RadixDialog.Root open={open} onOpenChange={onOpenChange}>
      <RadixDialog.Portal>
        <RadixDialog.Overlay className="fixed inset-0 z-50 bg-black/50" />
        {/*
          A column, so the *body* is what gives when the window is short: the header and the footer
          keep their height and everything left over is the body's, which then scrolls inside it.
          It used to be capped at 60vh of its own inside this 85vh — two caps, of which the smaller
          always won, so a dialog with a grid in it was squeezed on every screen and the room the
          shell had been given went unused.
        */}
        <RadixDialog.Content
          style={{ width }}
          className="fixed top-1/2 left-1/2 z-50 flex max-h-[85vh] max-w-[96vw] -translate-x-1/2 -translate-y-1/2 flex-col overflow-hidden rounded-lg border border-line-strong bg-elevated shadow-[var(--shadow-overlay)]"
        >
          <div className="flex shrink-0 items-center gap-3 border-b border-line bg-bar px-4 py-3">
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

          {/*
            `flex-auto` and not `flex-1`: the body's natural height is its basis, so a short dialog
            is still only as tall as it needs to be, and `min-h-0` is what lets it shrink below that
            height — without which it would push the footer off a short window instead of scrolling.
          */}
          <div className={cn('scroll-thin min-h-0 flex-auto overflow-y-auto', bodyClassName)}>
            {children}
          </div>

          {footer ? (
            <div className="flex shrink-0 items-center justify-end gap-2 border-t border-line bg-bar px-4 py-2.5">
              {footer}
            </div>
          ) : null}
        </RadixDialog.Content>
      </RadixDialog.Portal>
    </RadixDialog.Root>
  )
}
