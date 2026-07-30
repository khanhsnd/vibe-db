import {
  useEffect,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
  type MouseEvent as ReactMouseEvent,
  type PointerEvent as ReactPointerEvent,
} from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import { UnfoldHorizontal } from 'lucide-react'

import { copyText } from '@/lib/clipboard'
import { cn } from '@/lib/cn'
import { ROW_HEIGHT } from '@/shared/theme/density'

import { gridFont, widestText } from './measureText'
import {
  boundsOf,
  cellKey,
  cellText,
  isNumeric,
  toTSV,
  type Cell,
  type CellBounds,
  type CellRange,
  type CellRef,
  type GridData,
} from './resultSet'

const GUTTER_WIDTH = 52
const DEFAULT_COLUMN_WIDTH = 160
const MIN_COLUMN_WIDTH = 60
const MAX_FIT_WIDTH = 520
const CELL_PADDING = 17

const NOTHING_DIRTY: ReadonlySet<string> = new Set()

interface ResultGridProps {
  result: GridData
  selection?: CellRange | null
  onSelectionChange?: (selection: CellRange) => void
  /** Supplied only for a writable result; its absence keeps the grid read-only. */
  onEditCell?: ((ref: CellRef, value: Cell) => void) | undefined
  /** Cells carrying an unapplied edit, keyed by `cellKey`. */
  dirty?: ReadonlySet<string>
}

export function ResultGrid({
  result,
  selection,
  onSelectionChange,
  onEditCell,
  dirty = NOTHING_DIRTY,
}: ResultGridProps) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const dragging = useRef(false)
  const [resized, setResized] = useState<Record<string, number>>({})
  const [ownSelection, setOwnSelection] = useState<CellRange | null>(null)
  const [editing, setEditing] = useState<CellRef | null>(null)

  const current = selection === undefined ? ownSelection : selection

  const select = (next: CellRange) => {
    if (selection === undefined) setOwnSelection(next)
    onSelectionChange?.(next)
  }

  const widths = result.columns.map((column) => resized[column.name] ?? DEFAULT_COLUMN_WIDTH)

  const virtualizer = useVirtualizer({
    count: result.rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 16,
  })

  useEffect(() => {
    const stopDragging = () => {
      dragging.current = false
    }

    window.addEventListener('mouseup', stopDragging)
    return () => window.removeEventListener('mouseup', stopDragging)
  }, [])

  // Rows shift under the open input whenever a new result lands, so the editor closes with
  // it. The dependency is the row array, not the wrapper, which callers rebuild per render.
  useEffect(() => setEditing(null), [result.rows])

  const totalWidth = GUTTER_WIDTH + widths.reduce((sum, w) => sum + w, 0)
  const bounds = current ? boundsOf(current) : null

  const setWidth = (name: string, width: number) =>
    setResized((previous) => ({ ...previous, [name]: Math.max(MIN_COLUMN_WIDTH, width) }))

  const fitColumn = (index: number) => {
    const column = result.columns[index]
    if (!column) return

    const header = `${column.name}  ${column.type.toUpperCase()}`
    const texts = [header, ...result.rows.map((row) => cellText(row[index] ?? null))]
    const content = widestText(texts, gridFont()) + CELL_PADDING

    setWidth(column.name, Math.min(content, MAX_FIT_WIDTH))
  }

  const startSelect = (event: ReactMouseEvent, row: number, column: number) => {
    dragging.current = true
    const focus = { row, column }
    select(event.shiftKey && current ? { anchor: current.anchor, focus } : { anchor: focus, focus })
  }

  const extendSelect = (row: number, column: number) => {
    if (!dragging.current || !current) return
    if (current.focus.row === row && current.focus.column === column) return
    select({ anchor: current.anchor, focus: { row, column } })
  }

  const onKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (!current) return

    if (event.key === 'Enter' && onEditCell) {
      event.preventDefault()
      setEditing(current.anchor)
      return
    }
    if (!(event.ctrlKey || event.metaKey) || event.key.toLowerCase() !== 'c') return

    event.preventDefault()
    void copyText(toTSV(current, result))
  }

  return (
    <div
      ref={scrollRef}
      tabIndex={0}
      onKeyDown={onKeyDown}
      className="scroll-thin h-full overflow-auto bg-canvas"
    >
      <div className="w-full" style={{ minWidth: totalWidth }}>
        <HeaderRow
          columns={result.columns}
          widths={widths}
          onResize={setWidth}
          onFit={fitColumn}
        />

        <div className="relative" style={{ height: virtualizer.getTotalSize() }}>
          {virtualizer.getVirtualItems().map((item) => {
            const row = result.rows[item.index]
            if (!row) return null

            return (
              <div
                key={item.key}
                className="absolute top-0 left-0 flex w-full border-b border-line/60 hover:bg-hover"
                style={{ height: ROW_HEIGHT, transform: `translateY(${item.start}px)` }}
              >
                <div
                  className="sticky left-0 z-1 flex shrink-0 items-center justify-end border-r border-line bg-panel pr-2 font-mono text-2xs text-faint"
                  style={{ width: GUTTER_WIDTH }}
                >
                  {item.index + 1}
                </div>

                {result.columns.map((column, i) => {
                  const width = widths[i] ?? DEFAULT_COLUMN_WIDTH

                  if (onEditCell && editing?.row === item.index && editing.column === i) {
                    return (
                      <CellInput
                        key={i}
                        value={row[i] ?? null}
                        width={width}
                        onCommit={(value) => {
                          onEditCell({ row: item.index, column: i }, value)
                          setEditing(null)
                        }}
                        onCancel={() => setEditing(null)}
                      />
                    )
                  }

                  return (
                    <CellView
                      key={i}
                      value={row[i] ?? null}
                      numeric={isNumeric(column.type)}
                      width={width}
                      selected={within(bounds, item.index, i)}
                      anchored={current?.anchor.row === item.index && current.anchor.column === i}
                      edited={dirty.has(cellKey(item.index, i))}
                      onMouseDown={(event) => startSelect(event, item.index, i)}
                      onMouseOver={() => extendSelect(item.index, i)}
                      onDoubleClick={
                        onEditCell ? () => setEditing({ row: item.index, column: i }) : undefined
                      }
                    />
                  )
                })}
              </div>
            )
          })}
        </div>
      </div>
    </div>
  )
}

function within(bounds: CellBounds | null, row: number, column: number): boolean {
  return (
    bounds !== null &&
    row >= bounds.rowStart &&
    row <= bounds.rowEnd &&
    column >= bounds.columnStart &&
    column <= bounds.columnEnd
  )
}

interface HeaderRowProps {
  columns: GridData['columns']
  widths: number[]
  onResize: (name: string, width: number) => void
  onFit: (index: number) => void
}

function HeaderRow({ columns, widths, onResize, onFit }: HeaderRowProps) {
  return (
    <div className="sticky top-0 z-2 flex border-b border-line-strong bg-panel">
      <div
        className="sticky left-0 z-1 shrink-0 border-r border-line bg-panel"
        style={{ width: GUTTER_WIDTH, height: ROW_HEIGHT }}
      />

      {columns.map((column, i) => (
        <div
          key={i}
          className="relative flex shrink-0 items-center gap-1.5 border-r border-line px-2"
          style={{ width: widths[i] ?? DEFAULT_COLUMN_WIDTH, height: ROW_HEIGHT }}
        >
          <span className="truncate text-sm font-medium text-fg">{column.name}</span>
          <span className="ml-auto shrink-0 font-mono text-2xs text-faint uppercase">
            {column.type}
          </span>
          <ResizeHandle
            width={widths[i] ?? DEFAULT_COLUMN_WIDTH}
            onResize={(width) => onResize(column.name, width)}
            onFit={() => onFit(i)}
          />
        </div>
      ))}
    </div>
  )
}

interface ResizeHandleProps {
  width: number
  onResize: (width: number) => void
  onFit: () => void
}

function ResizeHandle({ width, onResize, onFit }: ResizeHandleProps) {
  const onPointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    event.stopPropagation()

    const startX = event.clientX
    const startWidth = width

    const onMove = (moveEvent: PointerEvent) => {
      onResize(startWidth + moveEvent.clientX - startX)
    }
    const onUp = () => {
      window.removeEventListener('pointermove', onMove)
      window.removeEventListener('pointerup', onUp)
    }

    window.addEventListener('pointermove', onMove)
    window.addEventListener('pointerup', onUp)
  }

  return (
    <div
      onPointerDown={onPointerDown}
      onDoubleClick={(event) => {
        event.stopPropagation()
        onFit()
      }}
      title="Drag to resize, double-click to fit contents"
      className="group/resize absolute top-0 -right-0.5 z-1 flex h-full w-1.5 cursor-col-resize justify-center"
    >
      <span className="h-full w-px group-hover/resize:bg-accent" />
      <UnfoldHorizontal
        size={13}
        className="pointer-events-none absolute top-1/2 -translate-y-1/2 text-accent opacity-0 group-hover/resize:opacity-100"
      />
    </div>
  )
}

interface CellViewProps {
  value: Cell
  numeric: boolean
  width: number
  selected: boolean
  anchored: boolean
  edited: boolean
  onMouseDown: (event: ReactMouseEvent) => void
  onMouseOver: () => void
  onDoubleClick: (() => void) | undefined
}

function CellView({
  value,
  numeric,
  width,
  selected,
  anchored,
  edited,
  onMouseDown,
  onMouseOver,
  onDoubleClick,
}: CellViewProps) {
  return (
    <div
      onMouseDown={onMouseDown}
      onMouseOver={onMouseOver}
      onDoubleClick={onDoubleClick}
      className={cn(
        'flex shrink-0 cursor-default items-center overflow-hidden border-r border-line/60 px-2',
        'font-mono text-sm whitespace-nowrap',
        numeric && 'justify-end',
        selected && 'bg-accent-subtle',
        anchored && 'shadow-[inset_0_0_0_1px_var(--accent)]',
        edited && 'shadow-[inset_0_0_0_1px_var(--warn)]',
      )}
      style={{ width }}
    >
      {value === null ? (
        <span className="text-2xs tracking-wide text-[var(--syn-null)] italic">NULL</span>
      ) : (
        <span className={cn('truncate', edited ? 'text-warn' : 'text-fg')}>{value}</span>
      )}
    </div>
  )
}

interface CellInputProps {
  value: Cell
  width: number
  onCommit: (value: Cell) => void
  onCancel: () => void
}

/** Leaving a NULL cell empty commits NULL again, so opening the editor never rewrites it. */
function CellInput({ value, width, onCommit, onCancel }: CellInputProps) {
  const [draft, setDraft] = useState(value ?? '')
  const commit = () => onCommit(draft === '' && value === null ? null : draft)

  return (
    <input
      autoFocus
      value={draft}
      spellCheck={false}
      onChange={(event) => setDraft(event.target.value)}
      onBlur={commit}
      onKeyDown={(event) => {
        if (event.key === 'Enter') commit()
        else if (event.key === 'Escape') onCancel()
        else return

        event.preventDefault()
        event.stopPropagation()
      }}
      style={{ width }}
      className="shrink-0 border-r border-line/60 bg-canvas px-2 font-mono text-sm text-fg shadow-[inset_0_0_0_1px_var(--accent)] outline-none"
    />
  )
}
