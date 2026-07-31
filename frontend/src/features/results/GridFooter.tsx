import type { ReactNode } from 'react'
import { Check, Minus, Plus, RotateCw, Square, X } from 'lucide-react'

import { IconButton } from '@/shared/ui/IconButton'

interface GridFooterProps {
  /** The four editing icons are hidden outright for rows that cannot be written back. */
  editable: boolean
  /** Delete needs a subject, which only a selection gives it. */
  selected: boolean
  /** Whether anything at all is waiting to be written. */
  pending: boolean
  busy: boolean
  onAddRow: () => void
  onDeleteRow: () => void
  onApply: () => void
  onDiscard: () => void
  onRefresh: () => void
  /** Only a running statement can be stopped; a grid without one leaves this out. */
  onStop?: (() => void) | undefined
  running?: boolean
  /** The right-hand side: a pager, a source label — whatever the surface counts in. */
  children?: ReactNode
}

/**
 * The action bar under every grid: what a row can be done to on the left, and what the
 * surface counts on the right. It carries its own surface colour so it reads as a bar of
 * controls rather than as another status line.
 */
export function GridFooter({
  editable,
  selected,
  pending,
  busy,
  onAddRow,
  onDeleteRow,
  onApply,
  onDiscard,
  onRefresh,
  onStop,
  running = false,
  children,
}: GridFooterProps) {
  return (
    <div
      role="toolbar"
      className="flex h-8 shrink-0 items-center gap-0.5 border-t border-line bg-footer px-1.5 text-xs text-muted"
    >
      {editable ? (
        <>
          <IconButton
            label="Add row"
            icon={<Plus size={14} />}
            disabled={busy}
            onClick={onAddRow}
          />
          <IconButton
            label="Delete row"
            icon={<Minus size={14} />}
            disabled={busy || !selected}
            onClick={onDeleteRow}
          />

          <span className="mx-1 h-4 w-px bg-line" />

          <IconButton
            label="Apply changes"
            icon={<Check size={14} />}
            disabled={busy || !pending}
            onClick={onApply}
          />
          <IconButton
            label="Discard changes"
            icon={<X size={14} />}
            disabled={busy || !pending}
            onClick={onDiscard}
          />
        </>
      ) : null}

      <IconButton
        label="Refresh"
        icon={<RotateCw size={13} />}
        disabled={busy}
        onClick={onRefresh}
      />

      {onStop ? (
        <IconButton
          label="Stop"
          icon={<Square size={11} />}
          disabled={!running}
          onClick={onStop}
        />
      ) : null}

      <div className="min-w-2 flex-1" />

      {children}
    </div>
  )
}
