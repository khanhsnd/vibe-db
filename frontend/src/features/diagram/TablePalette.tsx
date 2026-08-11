import { useMemo, useRef, useState, type PointerEvent as ReactPointerEvent } from 'react'
import { Check, Plus, Search, Table2 } from 'lucide-react'

import { cn } from '@/lib/cn'
import { Empty } from '@/shared/ui/Empty'
import { TOOLBAR_CONTROL } from '@/shared/ui/Field'
import { IconButton } from '@/shared/ui/IconButton'
import { Loading } from '@/shared/ui/Loading'

/**
 * Every table of the namespace, and how one gets onto the canvas.
 *
 * A table is **dragged** onto the picture, which is the one gesture that says where it should land —
 * and the plus beside it drops it in the first free space for a reader who does not care. Neither is
 * a single click: a press here picks the row and nothing more, which is what a press does everywhere
 * outside the grid.
 *
 * A table already drawn wears a tick and is not added twice; the plus becomes a way of finding it on
 * the canvas instead, since a diagram of forty boxes is a place a name is easy to lose.
 */
interface TablePaletteProps {
  tables: readonly string[]
  onCanvas: ReadonlySet<string>
  /** Whether the table list has been read at all — an unread namespace is waiting, not empty. */
  listed: boolean
  /** What this engine calls these things, for everything said about them here. */
  words: { table: string; tables: string }
  /** Adds a table, at the point it was dropped or — with no point — wherever there is room. */
  onAdd: (table: string, at: { clientX: number; clientY: number } | null) => void
  onReveal: (table: string) => void
}

/** How far the pointer travels before a press becomes a drag rather than a pick. */
const DRAG_THRESHOLD = 4

export function TablePalette({
  tables,
  onCanvas,
  listed,
  words,
  onAdd,
  onReveal,
}: TablePaletteProps) {
  const [filter, setFilter] = useState('')
  const [picked, setPicked] = useState('')
  const [ghost, setGhost] = useState<{ table: string; x: number; y: number } | null>(null)
  const start = useRef<{ table: string; x: number; y: number } | null>(null)

  const shown = useMemo(() => {
    const needle = filter.trim().toLowerCase()
    return needle === ''
      ? tables
      : tables.filter((table) => table.toLowerCase().includes(needle))
  }, [tables, filter])

  const onPointerDown = (event: ReactPointerEvent<HTMLDivElement>, table: string) => {
    if (event.button !== 0) return
    setPicked(table)
    if (onCanvas.has(table)) return

    start.current = { table, x: event.clientX, y: event.clientY }
    event.currentTarget.setPointerCapture(event.pointerId)
  }

  const onPointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    const held = start.current
    if (!held) return
    if (Math.hypot(event.clientX - held.x, event.clientY - held.y) < DRAG_THRESHOLD) return

    setGhost({ table: held.table, x: event.clientX, y: event.clientY })
  }

  const onPointerUp = (event: ReactPointerEvent<HTMLDivElement>) => {
    const held = start.current
    start.current = null
    setGhost(null)
    if (!held || ghost === null) return

    // Dropped on the canvas or nowhere: `elementFromPoint` is what says which, since the pointer is
    // captured here and the canvas never hears the press at all.
    const under = document.elementFromPoint(event.clientX, event.clientY)
    if (under?.closest('[data-diagram]')) {
      onAdd(held.table, { clientX: event.clientX, clientY: event.clientY })
    }
  }

  return (
    <div className="flex h-full min-h-0 flex-col border-r border-line bg-panel">
      <div className="flex h-8 shrink-0 items-center gap-1.5 border-b border-line bg-bar px-2">
        <Search size={12} className="shrink-0 text-faint" />
        <input
          value={filter}
          spellCheck={false}
          placeholder={`Filter ${words.tables.toLowerCase()}`}
          onChange={(event) => setFilter(event.target.value)}
          className={cn(TOOLBAR_CONTROL, 'w-full border-transparent bg-transparent px-0')}
        />
      </div>

      <div className="scroll-thin min-h-0 flex-1 overflow-y-auto py-1">
        {!listed ? (
          <Loading label={`Reading the ${words.tables.toLowerCase()}…`} />
        ) : shown.length === 0 ? (
          <Empty
            title={tables.length === 0 ? `No ${words.tables.toLowerCase()}` : 'Nothing matches'}
            hint={
              tables.length === 0
                ? 'This namespace holds none to draw.'
                : 'No name holds what was typed.'
            }
          />
        ) : (
          shown.map((table) => {
            const drawn = onCanvas.has(table)

            return (
              <div
                key={table}
                onPointerDown={(event) => onPointerDown(event, table)}
                onPointerMove={onPointerMove}
                onPointerUp={onPointerUp}
                onDoubleClick={() => (drawn ? onReveal(table) : onAdd(table, null))}
                className={cn(
                  'group flex h-6 cursor-default items-center gap-1.5 px-2 text-xs',
                  picked === table ? 'bg-selected text-fg' : 'text-muted hover:bg-hover',
                  !drawn && 'cursor-grab',
                )}
                title={drawn ? `${table} — already on the diagram` : `Drag ${table} onto the canvas`}
              >
                <Table2 size={12} className="shrink-0 text-kind-table" />
                <span className="min-w-0 flex-1 truncate">{table}</span>

                {drawn ? (
                  <Check size={12} className="shrink-0 text-success" />
                ) : (
                  <IconButton
                    label={`Add ${table}`}
                    icon={<Plus size={12} />}
                    onPointerDown={(event) => event.stopPropagation()}
                    onClick={() => onAdd(table, null)}
                    className="h-4 w-4 opacity-0 group-hover:opacity-100"
                  />
                )}
              </div>
            )
          })
        )}
      </div>

      <div className="flex h-6 shrink-0 items-center border-t border-line px-2 text-2xs text-faint">
        {`${onCanvas.size} of ${tables.length} drawn`}
      </div>

      {/* What is being dragged, following the pointer: the canvas never hears the press, so this is
          the only thing that says a table is on its way there. */}
      {ghost ? (
        <div
          className="pointer-events-none fixed z-50 flex items-center gap-1.5 rounded-md border border-accent bg-elevated px-2 py-1 text-xs text-fg shadow-[var(--shadow-overlay)]"
          style={{ left: ghost.x + 10, top: ghost.y + 10 }}
        >
          <Table2 size={12} className="text-kind-table" />
          {ghost.table}
        </div>
      ) : null}
    </div>
  )
}
