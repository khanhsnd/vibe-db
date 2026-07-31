import {
  useEffect,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
  type MouseEvent as ReactMouseEvent,
  type PointerEvent as ReactPointerEvent,
} from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import {
  ArrowDownNarrowWide,
  ArrowUpNarrowWide,
  EllipsisVertical,
  UnfoldHorizontal,
} from 'lucide-react'

import { copyText, readText } from '@/bridge/clipboard'
import { cn } from '@/lib/cn'
import { ROW_HEIGHT } from '@/shared/theme/density'
import { ContextMenu, type MenuItem } from '@/shared/ui/ContextMenu'
import { IconButton } from '@/shared/ui/IconButton'

import { ColumnTypeIcon, isNumeric } from './columnType'
import { gridFont, widestText } from './measureText'
import {
  boundsOf,
  cellText,
  columnRange,
  focusOf,
  oneRange,
  originOf,
  parseCell,
  toTSV,
  type Cell,
  type CellBounds,
  type CellRange,
  type CellRef,
  type GridData,
  type ResultColumn,
  type Selection,
  type SortKey,
} from './resultSet'

const GUTTER_WIDTH = 52
const HEADER_HEIGHT = 40
const DEFAULT_COLUMN_WIDTH = 160
const MIN_COLUMN_WIDTH = 60
const MAX_FIT_WIDTH = 520
const CELL_PADDING = 17
const TYPE_ICON_WIDTH = 17
/** The room the header's options button takes on the right, beyond a cell's own padding. */
const MENU_WIDTH = 20
/** Pixels scrolled per frame while a drag is held past an edge of the viewport. */
const EDGE_STEP = 14

const NOTHING_DIRTY = () => false
const NOTHING_DELETED = () => false

/** Where a drag started, so extending it never depends on the render that began it. */
interface Drag {
  anchor: CellRef
  focus: CellRef
  /** A drag begun in the row gutter selects whole rows, so its columns stay pinned. */
  wholeRow: boolean
  /** The rectangles the drag is not touching — everything a `Ctrl`+drag must leave standing. */
  kept: CellRange[]
}

interface Hit {
  ref: CellRef
  wholeRow: boolean
}

interface ResultGridProps {
  result: GridData
  selection?: Selection | null
  onSelectionChange?: (selection: Selection) => void
  /** Supplied only for a writable result; its absence keeps the grid read-only. */
  onEditCell?: ((ref: CellRef, value: Cell) => void) | undefined
  /**
   * Whether a cell carries an unapplied edit. A predicate rather than a set of keys, so a
   * call site that hides columns can answer in its own coordinates without rebuilding one.
   */
  isDirty?: (row: number, column: number) => boolean
  /** Whether a row is marked for deletion but not written back yet. A predicate for the same
   * reason `isDirty` is one: a call site that pages answers in its own coordinates. */
  isDeleted?: (row: number) => boolean
  /**
   * The column the rows arrived in order of, which the header marks. Reordering is the call
   * site's — a server-paged surface sorts in its statement, an in-memory one sorts its rows —
   * so the sort items appear on the header menu only where `onSortChange` was passed.
   */
  sort?: SortKey | null
  onSortChange?: (sort: SortKey | null) => void
  /**
   * Items appended to a header's context menu, for what only the call site can do — hiding a
   * column is its own state, not the grid's.
   */
  columnMenuItems?: (column: ResultColumn) => MenuItem[]
}

export function ResultGrid({
  result,
  selection,
  onSelectionChange,
  onEditCell,
  isDirty = NOTHING_DIRTY,
  isDeleted = NOTHING_DELETED,
  sort = null,
  onSortChange,
  columnMenuItems,
}: ResultGridProps) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const bodyRef = useRef<HTMLDivElement>(null)
  const drag = useRef<Drag | null>(null)
  const lastHit = useRef<Hit | null>(null)
  const pointer = useRef({ x: 0, y: 0 })
  const frame = useRef(0)
  const [resized, setResized] = useState<Record<string, number>>({})
  const [ownSelection, setOwnSelection] = useState<Selection | null>(null)
  const [editing, setEditing] = useState<CellRef | null>(null)

  const current = selection === undefined ? ownSelection : selection

  const select = (next: Selection) => {
    if (selection === undefined) setOwnSelection(next)
    onSelectionChange?.(next)
  }

  const widths = result.columns.map((column) => resized[column.name] ?? DEFAULT_COLUMN_WIDTH)
  const lastColumn = result.columns.length - 1

  const virtualizer = useVirtualizer({
    count: result.rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 16,
  })

  useEffect(() => () => cancelAnimationFrame(frame.current), [])

  // Rows shift under the open input whenever a new result lands, so the editor closes with
  // it. The dependency is the row array, not the wrapper, which callers rebuild per render.
  useEffect(() => setEditing(null), [result.rows])

  const totalWidth = GUTTER_WIDTH + widths.reduce((sum, w) => sum + w, 0)
  const bounds = current?.ranges.map(boundsOf) ?? []
  const focus = focusOf(current)

  const setWidth = (name: string, width: number) =>
    setResized((previous) => ({ ...previous, [name]: Math.max(MIN_COLUMN_WIDTH, width) }))

  const fitColumn = (index: number) => {
    const column = result.columns[index]
    if (!column) return

    const font = gridFont()
    const cells = result.rows.map((row) => cellText(row[index] ?? null))
    const content = Math.max(
      widestText(cells, font),
      widestText([column.name], font) + MENU_WIDTH,
      widestText([typeLabel(column)], font) + TYPE_ICON_WIDTH + MENU_WIDTH,
    )

    setWidth(column.name, Math.min(content + CELL_PADDING, MAX_FIT_WIDTH))
  }

  /** A header carries the whole column, so right-clicking one selects it before it acts. */
  const selectColumn = (index: number) => {
    if (result.rows.length > 0) select(oneRange(columnRange(index, result.rows.length)))
  }

  /**
   * Writes a clipboard block outwards from one cell — a line per row, a tab per column — and
   * never past the rows or columns the grid shows, so a paste can only overwrite what is there.
   * A copied row therefore goes back whole, which is what pasting into a fresh row means.
   */
  const paste = async (at: CellRef) => {
    const text = await readText()
    if (!onEditCell || text === '') return

    text.split(/\r?\n/).forEach((line, down) => {
      const row = at.row + down
      if (row >= result.rows.length) return

      line.split('\t').forEach((value, across) => {
        const column = at.column + across
        if (column < result.columns.length) onEditCell({ row, column }, parseCell(value))
      })
    })
  }

  const columnMenu = (index: number): MenuItem[] => {
    const column = result.columns[index]
    if (!column) return []

    const sorting: MenuItem[] = onSortChange
      ? [
          {
            label: 'Sort Ascending',
            onSelect: () => onSortChange({ column: column.name, descending: false }),
          },
          {
            label: 'Sort Descending',
            onSelect: () => onSortChange({ column: column.name, descending: true }),
          },
          {
            label: 'Remove All Sorts',
            disabled: sort === null,
            onSelect: () => onSortChange(null),
          },
        ]
      : []

    return [
      ...sorting,
      {
        label: 'Copy column',
        separatorBefore: sorting.length > 0,
        disabled: result.rows.length === 0,
        onSelect: () =>
          void copyText(toTSV(oneRange(columnRange(index, result.rows.length)), result)),
      },
      { label: 'Copy field name', onSelect: () => void copyText(column.name) },
      ...(onEditCell
        ? [
            {
              label: 'Paste',
              disabled: result.rows.length === 0,
              onSelect: () => void paste({ row: 0, column: index }),
            },
          ]
        : []),
      ...(columnMenuItems?.(column) ?? []),
    ]
  }

  /** Reads a cell straight off the pointer, so a drag needs no per-cell handlers. */
  const cellAt = (clientX: number, clientY: number, wholeRow: boolean): CellRef => {
    const rect = bodyRef.current?.getBoundingClientRect()
    const row = clamp(Math.floor((clientY - (rect?.top ?? 0)) / ROW_HEIGHT), result.rows.length - 1)
    const column = wholeRow
      ? lastColumn
      : columnAt(widths, clientX - (rect?.left ?? 0) - GUTTER_WIDTH)

    return { row, column }
  }

  const extend = (clientX: number, clientY: number) => {
    const state = drag.current
    if (!state) return

    const next = cellAt(clientX, clientY, state.wholeRow)
    if (next.row === state.focus.row && next.column === state.focus.column) return

    state.focus = next
    select({ ranges: [...state.kept, { anchor: state.anchor, focus: next }] })
  }

  const autoScroll = () => {
    const scroller = scrollRef.current
    if (!drag.current || !scroller) return

    const rect = scroller.getBoundingClientRect()
    const { x, y } = pointer.current
    const left = edgePush(x, rect.left, rect.right)
    const top = edgePush(y, rect.top + HEADER_HEIGHT, rect.bottom)

    if (left !== 0 || top !== 0) {
      scroller.scrollBy({ left, top })
      extend(x, y)
    }
    frame.current = requestAnimationFrame(autoScroll)
  }

  const onPointerDown = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (event.button !== 0 || event.target instanceof HTMLInputElement) return
    if (result.rows.length === 0) return

    const wholeRow = inGutter(event.target)
    const hit = cellAt(event.clientX, event.clientY, wholeRow)
    const live = current?.ranges.at(-1)

    // Shift reopens the live rectangle; Ctrl leaves every rectangle standing and starts a new
    // one beside them; a plain press starts over.
    const kept = event.shiftKey
      ? (current?.ranges.slice(0, -1) ?? [])
      : event.ctrlKey || event.metaKey
        ? (current?.ranges ?? [])
        : []
    const anchor =
      event.shiftKey && live ? live.anchor : wholeRow ? { row: hit.row, column: 0 } : hit

    drag.current = { anchor, focus: hit, wholeRow, kept }
    lastHit.current = { ref: hit, wholeRow }
    pointer.current = { x: event.clientX, y: event.clientY }
    select({ ranges: [...kept, { anchor, focus: hit }] })

    event.currentTarget.setPointerCapture(event.pointerId)
    scrollRef.current?.focus()
    frame.current = requestAnimationFrame(autoScroll)
  }

  const onPointerMove = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!drag.current) return
    pointer.current = { x: event.clientX, y: event.clientY }
    extend(event.clientX, event.clientY)
  }

  const endDrag = () => {
    drag.current = null
    cancelAnimationFrame(frame.current)
  }

  // Pointer capture retargets the click that follows it, so the cell comes from the press
  // that opened the double-click rather than from the event's own target.
  const onDoubleClick = () => {
    const hit = lastHit.current
    if (!onEditCell || !hit || hit.wholeRow) return
    setEditing(hit.ref)
  }

  const onKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    if (!current) return

    if (event.key === 'Enter' && onEditCell && focus) {
      event.preventDefault()
      setEditing(focus)
      return
    }
    if (!(event.ctrlKey || event.metaKey)) return

    const key = event.key.toLowerCase()
    const origin = originOf(current)

    if (key === 'c') {
      event.preventDefault()
      void copyText(toTSV(current, result))
    } else if (key === 'v' && onEditCell && origin) {
      event.preventDefault()
      void paste(origin)
    }
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
          bounds={bounds}
          sort={sort}
          onResize={setWidth}
          onFit={fitColumn}
          onSelectColumn={selectColumn}
          menuFor={columnMenu}
        />

        <div
          ref={bodyRef}
          onPointerDown={onPointerDown}
          onPointerMove={onPointerMove}
          onPointerUp={endDrag}
          onLostPointerCapture={endDrag}
          onDoubleClick={onDoubleClick}
          className="relative"
          style={{ height: virtualizer.getTotalSize() }}
        >
          {virtualizer.getVirtualItems().map((item) => {
            const row = result.rows[item.index]
            if (!row) return null

            // A row is banded across its width only when the selection covers the whole of it;
            // a column selection touches every row and must not paint the grid over.
            const banded = rowCovered(bounds, item.index, lastColumn)
            const touched = rowTouched(bounds, item.index)
            // A row on its way out reads as struck through whatever the selection says,
            // because the strike is inherited by every cell below it.
            const removed = isDeleted(item.index)

            return (
              <div
                key={item.key}
                className={cn(
                  'absolute top-0 left-0 flex w-full border-b border-line/60',
                  removed
                    ? 'bg-grid-row-deleted line-through'
                    : banded
                      ? 'bg-grid-row-selected'
                      : 'hover:bg-hover',
                )}
                style={{ height: ROW_HEIGHT, transform: `translateY(${item.start}px)` }}
              >
                <div
                  data-gutter=""
                  className={cn(
                    'sticky left-0 z-1 flex shrink-0 items-center justify-end border-r border-line pr-2 font-mono text-2xs',
                    removed
                      ? 'bg-grid-row-deleted text-danger'
                      : touched
                        ? 'bg-grid-row-selected text-fg shadow-[inset_2px_0_0_var(--accent)]'
                        : 'bg-panel text-faint',
                  )}
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
                      inRange={within(bounds, item.index, i)}
                      focused={focus?.row === item.index && focus.column === i}
                      edited={isDirty(item.index, i)}
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

const typeLabel = (column: ResultColumn) => column.fullType || column.type

const inGutter = (target: EventTarget) =>
  target instanceof Element && target.closest('[data-gutter]') !== null

const clamp = (value: number, highest: number) => Math.min(Math.max(value, 0), Math.max(highest, 0))

function columnAt(widths: number[], x: number): number {
  let edge = 0
  for (let i = 0; i < widths.length; i++) {
    edge += widths[i] ?? 0
    if (x < edge) return i
  }
  return Math.max(widths.length - 1, 0)
}

/**
 * Scrolls only once the drag is genuinely past the edge — a press that merely lands near one
 * must not move the grid under the pointer.
 */
function edgePush(position: number, start: number, end: number): number {
  if (position < start) return -EDGE_STEP
  if (position > end) return EDGE_STEP
  return 0
}

/** A cell is selected when any one rectangle covers it; likewise a banded row or header. */
function within(bounds: CellBounds[], row: number, column: number): boolean {
  return bounds.some(
    (one) =>
      row >= one.rowStart &&
      row <= one.rowEnd &&
      column >= one.columnStart &&
      column <= one.columnEnd,
  )
}

function rowTouched(bounds: CellBounds[], row: number): boolean {
  return bounds.some((one) => row >= one.rowStart && row <= one.rowEnd)
}

/** Whether one rectangle holds the whole row — every column of it, not merely some. */
function rowCovered(bounds: CellBounds[], row: number, lastColumn: number): boolean {
  return bounds.some(
    (one) =>
      row >= one.rowStart &&
      row <= one.rowEnd &&
      one.columnStart === 0 &&
      one.columnEnd >= lastColumn,
  )
}

function columnTouched(bounds: CellBounds[], column: number): boolean {
  return bounds.some((one) => column >= one.columnStart && column <= one.columnEnd)
}

interface HeaderRowProps {
  columns: GridData['columns']
  widths: number[]
  bounds: CellBounds[]
  sort: SortKey | null
  onResize: (name: string, width: number) => void
  onFit: (index: number) => void
  onSelectColumn: (index: number) => void
  menuFor: (index: number) => MenuItem[]
}

function HeaderRow({
  columns,
  widths,
  bounds,
  sort,
  onResize,
  onFit,
  onSelectColumn,
  menuFor,
}: HeaderRowProps) {
  return (
    <div className="sticky top-0 z-2 flex border-b border-line-strong bg-panel">
      <div
        className="sticky left-0 z-1 shrink-0 border-r border-line bg-panel"
        style={{ width: GUTTER_WIDTH, height: HEADER_HEIGHT }}
      />

      {columns.map((column, i) => {
        const active = columnTouched(bounds, i)
        const sorted = sort?.column === column.name ? sort : null

        return (
          <ContextMenu key={i} items={menuFor(i)}>
            <div
              onContextMenu={() => onSelectColumn(i)}
              className={cn(
                'relative flex shrink-0 flex-col justify-center border-r border-line pr-7 pl-2',
                active ? 'bg-grid-cell-selected' : 'bg-panel',
              )}
              style={{ width: widths[i] ?? DEFAULT_COLUMN_WIDTH, height: HEADER_HEIGHT }}
            >
              <span className="flex items-center gap-1">
                <span className="min-w-0 truncate font-mono text-sm font-semibold text-fg">
                  {column.name}
                </span>
                {sorted ? <SortMark descending={sorted.descending} /> : null}
              </span>
              <span className="flex items-center gap-1 text-2xs text-muted">
                <ColumnTypeIcon type={column.type} />
                <span className="truncate font-mono">{typeLabel(column)}</span>
              </span>

              <IconButton
                label={`${column.name} options`}
                icon={<EllipsisVertical size={14} />}
                onMouseDown={(event) => event.preventDefault()}
                onClick={openColumnMenu}
                className="absolute top-1/2 right-1 z-1 -translate-y-1/2"
              />

              <ResizeHandle
                width={widths[i] ?? DEFAULT_COLUMN_WIDTH}
                onResize={(width) => onResize(column.name, width)}
                onFit={() => onFit(i)}
              />
            </div>
          </ContextMenu>
        )
      })}
    </div>
  )
}

/**
 * Opens the header's own menu from a left click, by handing Radix the event it already
 * listens for. One menu with two ways in — the alternative is a second menu beside it that
 * drifts from the first, and the column is selected either way.
 */
function openColumnMenu(event: ReactMouseEvent<HTMLButtonElement>) {
  const { left, bottom } = event.currentTarget.getBoundingClientRect()

  event.currentTarget.dispatchEvent(
    new MouseEvent('contextmenu', {
      bubbles: true,
      cancelable: true,
      clientX: left,
      clientY: bottom,
    }),
  )
}

function SortMark({ descending }: { descending: boolean }) {
  const Icon = descending ? ArrowDownNarrowWide : ArrowUpNarrowWide
  return <Icon size={12} className="shrink-0 text-accent" />
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
  inRange: boolean
  focused: boolean
  edited: boolean
}

function CellView({ value, numeric, width, inRange, focused, edited }: CellViewProps) {
  return (
    <div
      className={cn(
        'flex shrink-0 items-center overflow-hidden border-r border-line/60 px-2',
        'font-mono text-sm whitespace-nowrap',
        numeric && 'justify-end',
        focused ? 'bg-accent' : inRange && 'bg-grid-cell-selected',
        edited && 'shadow-[inset_0_0_0_1px_var(--warn)]',
      )}
      style={{ width }}
    >
      <span
        className={cn(
          'truncate',
          focused
            ? 'text-on-accent'
            : value === null
              ? 'text-null'
              : edited
                ? 'text-warn'
                : 'text-fg',
        )}
      >
        {cellText(value)}
      </span>
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
