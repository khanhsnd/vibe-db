import { useMemo, useState, type ReactNode } from 'react'
import * as Popover from '@radix-ui/react-popover'
import { Check, Search, Square, SquareCheck } from 'lucide-react'

import { cn } from '@/lib/cn'
import { MENU_SURFACE } from '@/shared/ui/ContextMenu'
import { TOOLBAR_CONTROL } from '@/shared/ui/Field'

/** How tall a list may grow before it scrolls. A picker is a small panel, not a page. */
const LIST_HEIGHT = 188

/** Room for the search glyph, in a style rather than a class — see `BOX`. */
const SEARCH_INSET = 24

/**
 * Every part of a condition wears this, so the three read as one control split in three rather
 * than as three controls that happen to sit together.
 */
export const PART = 'shrink-0 truncate rounded px-1 text-xs hover:bg-hover'

/**
 * A box inside a picker. `TOOLBAR_CONTROL` already sizes a control for a strip rather than a form,
 * which is what a box on a 28 px row is; anything that needs its own padding sets it in a style,
 * because a second padding utility here would be settled by the stylesheet's order.
 */
const BOX = `${TOOLBAR_CONTROL} min-w-0 flex-1`

interface PickerPanelProps {
  /** What the part is, for a reader who cannot see where the panel hangs from. */
  label: string
  /** The part itself, which is the trigger. */
  text: string
  /** Accent for the reader's own two ends of a condition, plain for the comparison between them. */
  tone: 'accent' | 'plain'
  width?: number
  open: boolean
  onOpenChange: (open: boolean) => void
  children: ReactNode
}

/**
 * A small panel hung under one part of a condition, rather than a dialog over the grid: what is
 * being changed is three words on a row, and a modal that hides the table to change one of them
 * costs the reader the rows they are narrowing. It wears the menus' own surface, so a part that
 * opens a list and a part that opens a menu read as the same kind of control.
 */
export function PickerPanel({
  label,
  text,
  tone,
  width = 232,
  open,
  onOpenChange,
  children,
}: PickerPanelProps) {
  return (
    <Popover.Root open={open} onOpenChange={onOpenChange}>
      <Popover.Trigger asChild>
        <button
          type="button"
          aria-label={label}
          className={cn(PART, 'max-w-72', tone === 'accent' ? 'text-accent' : 'text-fg')}
        >
          {text}
        </button>
      </Popover.Trigger>

      <Popover.Portal>
        <Popover.Content
          align="start"
          sideOffset={4}
          style={{ width }}
          className={cn(MENU_SURFACE, 'p-1.5')}
        >
          {children}
        </Popover.Content>
      </Popover.Portal>
    </Popover.Root>
  )
}

interface SearchListProps {
  options: readonly string[]
  /** Which of them are taken, ticked in the list. A single-valued part passes the one it holds. */
  picked: ReadonlySet<string>
  onPick: (option: string) => void
  /** True while the options are still being read, which a column's values are on every open. */
  loading?: boolean
  /** What to say when the list is genuinely empty rather than searched down to nothing. */
  emptyLabel: string
  /** Whether the search box takes the focus, which it should wherever it is the point of the panel. */
  autoFocus?: boolean
  /**
   * Whether more than one may be taken, which decides what a row is marked with: a **check box**
   * where a second tick adds to the first, and a bare tick where it would replace it. The box is
   * the grid's own — see `CheckMark` — so one glyph means one thing everywhere.
   */
  multiple?: boolean
}

/**
 * A list of what a part may be, with a **search over it**: a table has as many columns as it likes
 * and a column as many values as its rows, so neither is a list a reader should have to scroll to
 * find a name they already know.
 */
export function SearchList({
  options,
  picked,
  onPick,
  loading = false,
  emptyLabel,
  autoFocus = false,
  multiple = false,
}: SearchListProps) {
  const [needle, setNeedle] = useState('')

  const matches = useMemo(() => {
    const search = needle.trim().toLowerCase()
    return options.filter((one) => one.toLowerCase().includes(search))
  }, [options, needle])

  return (
    <div className="flex flex-col gap-1">
      <div className="relative">
        <Search size={12} className="absolute top-1/2 left-2 -translate-y-1/2 text-faint" />
        <input
          autoFocus={autoFocus}
          value={needle}
          placeholder="Search"
          spellCheck={false}
          onChange={(event) => setNeedle(event.target.value)}
          style={{ paddingLeft: SEARCH_INSET }}
          className={cn(TOOLBAR_CONTROL, 'w-full placeholder:text-faint')}
        />
      </div>

      <div className="scroll-thin overflow-y-auto" style={{ maxHeight: LIST_HEIGHT }}>
        {loading ? (
          <Note>Reading the column…</Note>
        ) : matches.length === 0 ? (
          <Note>{options.length === 0 ? emptyLabel : 'Nothing matches that search.'}</Note>
        ) : (
          matches.map((option) => (
            <button
              key={option}
              type="button"
              role={multiple ? 'checkbox' : 'option'}
              aria-checked={multiple ? picked.has(option) : undefined}
              aria-selected={multiple ? undefined : picked.has(option)}
              onClick={() => onPick(option)}
              className={cn(
                'flex w-full items-center gap-1.5 rounded px-1 py-1 text-left hover:bg-hover',
                picked.has(option) ? 'text-fg' : 'text-muted',
              )}
            >
              <Mark on={picked.has(option)} multiple={multiple} />
              <span className="data-font min-w-0 truncate">{option}</span>
            </button>
          ))
        )}
      </div>
    </div>
  )
}

const Note = ({ children }: { children: ReactNode }) => (
  <p className="px-1 py-1 text-xs text-faint">{children}</p>
)

/** A box where a second tick adds, a bare tick where it replaces — the row says which it is. */
function Mark({ on, multiple }: { on: boolean; multiple: boolean }) {
  const Glyph = multiple ? (on ? SquareCheck : Square) : on ? Check : null

  return (
    <span className="grid h-3.5 w-3.5 shrink-0 place-items-center">
      {Glyph ? <Glyph size={13} className={on ? 'text-accent' : 'text-faint'} /> : null}
    </span>
  )
}

interface FieldPickProps {
  names: readonly string[]
  value: string
  onChange: (column: string) => void
}

/**
 * Which column a condition or a sort key names. One value, so picking closes the panel: there is
 * nothing left to ask.
 */
export function FieldPick({ names, value, onChange }: FieldPickProps) {
  const [open, setOpen] = useState(false)

  return (
    <PickerPanel label="Column" text={value} tone="accent" open={open} onOpenChange={setOpen}>
      <SearchList
        autoFocus
        options={names}
        picked={new Set([value])}
        emptyLabel="This result has no columns."
        onPick={(column) => {
          onChange(column)
          setOpen(false)
        }}
      />
    </PickerPanel>
  )
}

export { BOX }
