import { useState } from 'react'
import * as Popover from '@radix-ui/react-popover'
import { Filter } from 'lucide-react'

import type { TableColumn } from '@/bridge/tables'
import { cn } from '@/lib/cn'
import { Button } from '@/shared/ui/Button'
import { MENU_SURFACE } from '@/shared/ui/ContextMenu'
import { TOOLBAR_CONTROL } from '@/shared/ui/Field'

import type { FieldFilters } from './fieldFilter'

const PANEL_WIDTH = 296
/** How tall the list of fields grows before it scrolls. A panel is not a page. */
const LIST_HEIGHT = 216
/** The width the field names share, so every box down the panel starts at one place. */
const NAME_WIDTH = 92

interface FieldFilterButtonProps {
  /** The compared columns, and the only ones there is anything to narrow by. */
  columns: readonly TableColumn[]
  filters: FieldFilters
  /**
   * How many of them are in force, drawn on the button: shutting the panel does not widen the
   * panes, and a button saying nothing would suggest it did.
   */
  count: number
  /** How many pairs are drawn of how many were read, which the rows themselves cannot say. */
  shown: number
  total: number
  onChange: (column: string, text: string) => void
  onClear: () => void
}

/**
 * The funnel over a data comparison, and the panel it opens: **a box per compared field**.
 *
 * It is a panel hung off the toolbar rather than a dialog, for the reason every filter in this app
 * is one — what the reader is doing is watching two panes narrow as they type, and a modal would
 * cover the very rows they are aiming at. Every keystroke narrows and every keystroke is kept, so
 * there is nothing to apply: the panel shuts on `Escape`, on `Enter` and on a press outside it, and
 * the boxes hold until they are cleared.
 */
export function FieldFilterButton({
  columns,
  filters,
  count,
  shown,
  total,
  onChange,
  onClear,
}: FieldFilterButtonProps) {
  const [open, setOpen] = useState(false)

  return (
    <Popover.Root open={open} onOpenChange={setOpen}>
      <Popover.Trigger asChild>
        <Button
          variant="subtle"
          icon={<Filter size={13} className="text-accent" />}
          aria-pressed={count > 0}
          title="Show only the rows holding text in a field"
          className={cn(count > 0 || open ? 'bg-selected text-fg' : undefined)}
        >
          Fields
          {count > 0 ? (
            <span className="rounded-full bg-accent px-1.5 text-2xs text-on-accent">{count}</span>
          ) : null}
        </Button>
      </Popover.Trigger>

      <Popover.Portal>
        <Popover.Content
          align="start"
          sideOffset={4}
          style={{ width: PANEL_WIDTH }}
          className={cn(MENU_SURFACE, 'flex flex-col gap-1.5 p-1.5')}
        >
          <div
            className="scroll-thin flex flex-col gap-1 overflow-y-auto"
            style={{ maxHeight: LIST_HEIGHT }}
          >
            {columns.map((column, at) => (
              <label key={column.name} className="flex items-center gap-1.5">
                <span
                  title={`${column.name} ${column.fullType}`}
                  style={{ width: NAME_WIDTH }}
                  className="shrink-0 truncate font-mono text-2xs text-muted"
                >
                  {column.name}
                </span>
                <input
                  autoFocus={at === 0}
                  value={filters[column.name] ?? ''}
                  onChange={(event) => onChange(column.name, event.target.value)}
                  onKeyDown={(event) => {
                    if (event.key === 'Enter') setOpen(false)
                  }}
                  placeholder="Holds text"
                  spellCheck={false}
                  className={cn(TOOLBAR_CONTROL, 'min-w-0 flex-1 placeholder:text-faint')}
                />
              </label>
            ))}
          </div>

          <div className="flex items-center gap-2 border-t border-line pt-1.5">
            <p className="selectable min-w-0 flex-1 text-2xs text-faint">
              {note(count, shown, total)}
            </p>
            {count > 0 ? (
              <button
                type="button"
                onClick={onClear}
                className="shrink-0 rounded px-1 text-2xs text-muted hover:bg-hover hover:text-fg"
              >
                Clear all
              </button>
            ) : null}
          </div>
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}

/**
 * What the panel says about the boxes in it. Nothing matching is the one answer worth spelling out:
 * two empty panes are otherwise read as two tables that agreed.
 */
function note(count: number, shown: number, total: number): string {
  if (count === 0) return 'Every row read is drawn until something is typed.'
  if (shown === 0) return 'Nothing matches — the panes are narrowed, not the tables empty.'

  return `${shown} of the ${total} rows read are drawn.`
}
