import { useState } from 'react'
import * as Popover from '@radix-ui/react-popover'
import { Filter, X } from 'lucide-react'

import { cn } from '@/lib/cn'
import { MENU_SURFACE } from '@/shared/ui/ContextMenu'

const PANEL_WIDTH = 232

interface DatabaseFilterButtonProps {
  pattern: string
  /**
   * How many databases the branch is drawing, which is the one thing the rows cannot say — and
   * `null` while the connection is shut, where there is no list yet for a pattern to narrow. A
   * filter is still worth setting there: it is what the branch opens with once the server answers.
   */
  shown: number | null
  onChange: (pattern: string) => void
}

/**
 * The funnel on a connection row and the panel it opens: which of that connection's databases the
 * tree draws.
 *
 * It is a **panel hung off the row** rather than a dialog, for the reason a filter on a grid is one:
 * what the reader is doing is watching a list narrow as they type, and a modal over the sidebar would
 * cover the very rows they are aiming at. It narrows on every keystroke and every keystroke is kept,
 * so there is nothing to confirm — the panel closes on `Escape`, on `Enter` and on a press anywhere
 * outside it, and the pattern stays until it is cleared.
 */
export function DatabaseFilterButton({ pattern, shown, onChange }: DatabaseFilterButtonProps) {
  const [open, setOpen] = useState(false)
  const on = pattern.trim() !== ''

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <button
          type="button"
          aria-label="Filter databases"
          title={
            on
              ? `Showing databases matching “${pattern}”`
              : 'Show only databases matching a name'
          }
          // The row under it selects on a press and opens on two, and neither is what reaching for
          // this control means. Radix still hears the press: stopping the bubble is not preventing.
          onClick={(e) => e.stopPropagation()}
          onDoubleClick={(e) => e.stopPropagation()}
          className={cn(
            'grid h-4 w-4 shrink-0 place-items-center',
            // Off an unfiltered connection the funnel is a hover control, so a sidebar of servers
            // reads as names rather than as a column of buttons. On a filtered one it stays lit: a
            // branch drawing four of forty databases has to say why, and one drawing none must.
            on || open
              ? 'text-accent'
              : 'text-faint opacity-0 group-hover:opacity-100 hover:text-fg',
          )}
        >
          <Filter size={11} />
        </button>
      </Popover.Trigger>

      <Popover.Portal>
        <Popover.Content
          align="end"
          sideOffset={4}
          style={{ width: PANEL_WIDTH }}
          className={cn(MENU_SURFACE, 'flex flex-col gap-1.5 p-1.5')}
        >
          {/* The sidebar's own filter box, because it is one: a reader who has met that box at the
              top of the tree has met this one. */}
          <div className="flex h-6 items-center gap-1.5 rounded-md border border-line bg-canvas px-1.5 transition-colors focus-within:border-accent focus-within:ring-1 focus-within:ring-accent">
            <Filter size={11} className="shrink-0 text-faint" />
            <input
              autoFocus
              value={pattern}
              onChange={(e) => onChange(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Enter') setOpen(false)
              }}
              placeholder="Part of a name"
              spellCheck={false}
              className="min-w-0 flex-1 bg-transparent text-xs text-fg outline-none placeholder:text-faint"
            />
            {on ? (
              <button
                type="button"
                aria-label="Clear filter"
                onClick={() => onChange('')}
                className="grid shrink-0 place-items-center text-faint hover:text-fg"
              >
                <X size={11} />
              </button>
            ) : null}
          </div>

          <p className="selectable px-0.5 text-2xs text-faint">{note(on, shown)}</p>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}

/**
 * What the panel says about the pattern in it. A **shut connection** is the one case the number
 * cannot answer: an empty branch there is a server nobody has dialled, not a pattern that matched
 * nothing, and the two look exactly alike in the tree.
 */
function note(on: boolean, shown: number | null): string {
  if (!on) return 'Every database is shown until something is typed.'
  if (shown === null) return 'It applies when this connection is opened.'
  if (shown === 0) return 'Nothing matches — the branch is empty, not the server.'

  return `${shown} of the databases shown.`
}
