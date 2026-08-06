import {
  useCallback,
  useEffect,
  useMemo,
  useRef,
  useState,
  type KeyboardEvent as ReactKeyboardEvent,
  type MouseEvent as ReactMouseEvent,
  type PointerEvent as ReactPointerEvent,
  type RefObject,
} from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import {
  ArrowDownNarrowWide,
  ArrowUpNarrowWide,
  ChevronDown,
  ChevronRight,
  EllipsisVertical,
  KeyRound,
  Square,
  SquareCheck,
  UnfoldHorizontal,
} from 'lucide-react'

import { copyText, readText } from '@/bridge/clipboard'
import type { ExportFormat } from '@/bridge/export'
import { cn } from '@/lib/cn'
import {
  DENSE_GUTTER_WIDTH,
  DENSE_HEADER_HEIGHT,
  DENSE_ROW_HEIGHT,
  ROW_HEIGHT,
} from '@/shared/theme/density'
import { ContextMenu, type MenuItem } from '@/shared/ui/ContextMenu'
import { IconButton } from '@/shared/ui/IconButton'
import { ICON } from '@/shared/ui/menuIcons'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'

import { CellChoice } from './CellChoice'
import { CellEditorButton } from './CellEditorButton'
import { CellPicker } from './CellPicker'
import { ExportMenu } from './ExportMenu'
import { FilterSortButton } from './FilterSortButton'
import { FilterSortPanel, type FilterAsk } from './FilterSortPanel'
import { cellMenuItems } from './cellMenu'
import { ValueDialog } from './ValueDialog'
import { ColumnTypeIcon, isNumeric, temporalPicker, type TemporalPicker } from './columnType'
import { countConditions, type GridFiltering } from './filter'
import { gridFont, headerFont, widestText } from './measureText'
import {
  boundsOf,
  cellPreview,
  checked,
  CHECK_ON,
  columnRange,
  focusOf,
  isToggle,
  oneRange,
  originOf,
  parseCell,
  NO_SORT,
  rowRange,
  toTSV,
  type Cell,
  type CellBounds,
  type CellRange,
  type CellRef,
  type ColumnEditor,
  type DiffMark,
  type GridData,
  type GridOutline,
  type ResultColumn,
  type Selection,
  type SortKey,
} from './resultSet'

const GUTTER_WIDTH = 52
const HEADER_HEIGHT = 40

/**
 * How tall a row, its header and its gutter are. There are two sets of them and no more: the one
 * every grid has, and the one a grid drawn to be *scanned* takes — see `density.ts`. The numbers
 * travel as an object rather than as three arguments because they are read in seven places and
 * three of them are outside the component.
 */
interface Density {
  row: number
  header: number
  gutter: number
}

const ROOMY: Density = { row: ROW_HEIGHT, header: HEADER_HEIGHT, gutter: GUTTER_WIDTH }
const DENSE: Density = {
  row: DENSE_ROW_HEIGHT,
  header: DENSE_HEADER_HEIGHT,
  gutter: DENSE_GUTTER_WIDTH,
}
const DEFAULT_COLUMN_WIDTH = 160
const MIN_COLUMN_WIDTH = 60
const MAX_FIT_WIDTH = 520
const CELL_PADDING = 17
const TYPE_ICON_WIDTH = 17
/** The room the lock on a key column takes, ahead of that column's name. */
const KEY_ICON_WIDTH = 15
/** The room the header's options button takes on the right, beyond a cell's own padding. */
const MENU_WIDTH = 20
/** How far one level of an outlined column indents its name — the tree's own step. */
const OUTLINE_INDENT = 12
/** Pixels scrolled per frame while a drag is held past an edge of the viewport. */
const EDGE_STEP = 14
/**
 * How many rows a fit measures. A grid may hold every row of a table — a million of them — and
 * measuring each one costs seconds of frozen window for a width nobody can see anyway.
 */
const FIT_SAMPLE = 2000
/**
 * What the Filter & Sort panel opens at, and how far it may be dragged. Only the size it opens at
 * is a look — the bounds are what stop a drag leaving the panel too small to build a condition in
 * or the rows with nothing left.
 */
const FILTER_HEIGHT = 200
const FILTER_MIN_HEIGHT = 116

/** Where each arrow key takes the focus, in cells. */
const ARROWS: Record<string, CellRef> = {
  ArrowUp: { row: -1, column: 0 },
  ArrowDown: { row: 1, column: 0 },
  ArrowLeft: { row: 0, column: -1 },
  ArrowRight: { row: 0, column: 1 },
}

const NOTHING_DIRTY = () => false
const NOTHING_DELETED = () => false
const NOTHING_MARKED = () => undefined
const NO_CELL_MARKED = () => false

/**
 * What a comparison paints a row in: the row's own wash, its gutter's, and the bar down the
 * gutter's leading edge that says which of the four it is without reading a legend. The colours
 * are `--grid-diff-*` tokens and never a hex — see `shared/theme/tokens.css`.
 */
const DIFF_ROW: Record<DiffMark, string> = {
  added: 'bg-grid-diff-added',
  removed: 'bg-grid-diff-removed',
  changed: 'bg-grid-diff-changed',
  absent: 'bg-grid-diff-absent',
}

const DIFF_GUTTER: Record<DiffMark, string> = {
  added: 'bg-grid-diff-added text-diff-added shadow-[inset_2px_0_0_var(--diff-added)]',
  removed: 'bg-grid-diff-removed text-diff-removed shadow-[inset_2px_0_0_var(--diff-removed)]',
  changed: 'bg-grid-diff-changed text-diff-changed shadow-[inset_2px_0_0_var(--diff-changed)]',
  absent: 'bg-grid-diff-absent text-faint',
}

/** Where a drag started, so extending it never depends on the render that began it. */
interface Drag {
  anchor: CellRef
  focus: CellRef
  /** A drag begun in the row gutter selects whole rows, so its columns stay pinned. */
  wholeRow: boolean
  /** The rectangles the drag is not touching — everything a `Ctrl`+drag must leave standing. */
  kept: CellRange[]
}

/**
 * The press a click belongs to. Everything a click needs is read here, at the press, because
 * focusing the grid can scroll it and a measurement taken again on the click would then name a
 * different cell — which is why one click on a fresh grid used to do nothing at all.
 */
interface Hit {
  ref: CellRef
  wholeRow: boolean
  /** A press that was reaching for a range rather than for the cell under it. */
  extending: boolean
  /** Set once the drag leaves the cell it began on: a drag is not a click on that cell. */
  moved: boolean
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
   * What a comparison says about a row, where the grid is drawing one side of one — and nothing
   * at all for a row the two sides agree about. A predicate for the reason the two above are:
   * a comparison holds its own list of pairs and answers in the grid's coordinates from it.
   *
   * Deletion wins over it, being the more consequential fact, and neither surface has both.
   */
  markRow?: (row: number) => DiffMark | undefined
  /**
   * Whether this cell is one of the ones that differ, which a `changed` row draws cell by cell so
   * a row of forty columns says *which* of them moved. It is asked only of a marked row.
   */
  markCell?: (row: number, column: number) => boolean
  /**
   * The keys the rows arrived in order of, which the header marks — the first deciding and the
   * rest breaking its ties, so a header carrying the second key says so with its place. Reordering
   * is the call site's — a server-paged surface sorts in its statement, an in-memory one sorts its
   * rows — so the sort items appear on the header menu only where `onSortChange` was passed.
   */
  sort?: readonly SortKey[]
  onSortChange?: (sort: SortKey[]) => void
  /**
   * What a double-click on a row opens, where the rows stand for something that can be opened —
   * a table of the table list. Only a read-only grid is offered it: in a writable one a click is
   * already an editor, and the second one would land inside it.
   */
  onActivateRow?: ((row: number) => void) | undefined
  /**
   * Items appended to a header's context menu, for what only the call site can do — hiding a
   * column is its own state, not the grid's.
   */
  columnMenuItems?: (column: ResultColumn) => MenuItem[]
  /**
   * Moves a column, where the surface lets its reader arrange them: a header is dragged and this
   * says where it was dropped — `from` is the column being carried and `to` is the place it landed
   * **before**, `columns.length` meaning past the last one. Both are the grid's own positions, as
   * every other callback reports them, and **who keeps the order is the call site**: only it knows
   * whether the order outlives the grid. Its absence leaves the headers unmovable.
   */
  onMoveColumn?: ((from: number, to: number) => void) | undefined
  /**
   * Writes every row the grid holds to a file, in the order it draws them. A grid with nothing
   * to write leaves this out, which is what keeps the button off an empty result.
   */
  onExport?: ((format: ExportFormat) => void) | undefined
  /**
   * Whether the grid draws its own head strip — the **Filter & Sort** button on the left, Export
   * against the right. A surface with a toolbar of its own puts both there and passes `false`, so
   * one table never carries two strips, while the cell menu still reaches the same handlers
   * wherever the buttons live.
   */
  headStrip?: boolean
  /**
   * Opens the surface's own cell editor on a cell — the panel under `TableDataView`. A surface
   * without one leaves this out and the cell menu then offers no such item.
   */
  onEditInPanel?: ((ref: CellRef) => void) | undefined
  /**
   * That same panel's toggle, for the head strip to carry beside Export — whether it is open, and
   * what opens it. A surface with a toolbar of its own draws the same button there instead and
   * passes `headStrip={false}`, so one table never carries two of them.
   */
  cellPanel?: { shown: boolean; onToggle: () => void } | undefined
  /**
   * Marks rows for deletion: the same call the footer's Delete makes, from the cell menu instead.
   * Rows are the grid's own numbers, as every other callback reports them.
   */
  onDeleteRows?: ((rows: number[]) => void) | undefined
  /** Reads the rows again — the footer's Refresh, offered on the cell menu too. */
  onRefresh?: (() => void) | undefined
  /**
   * The conditions the rows are narrowed by, where the surface can narrow them: a table read
   * puts them in its `WHERE`, a query result applies them to the rows it already holds.
   */
  filtering?: GridFiltering | undefined
  /**
   * Lays a value out for the value viewer, in the engine's own notation — the driver's `Format`,
   * because **how text is laid out is the engine's** and a document store's documents are not JSON.
   * A surface with a session passes it; one without leaves it out, and a document then reads in
   * `Raw` alone. See `ValueViewer.formatValue`, which requires it to be referentially stable.
   */
  formatValue?: ((value: string) => Promise<string>) | undefined
  /**
   * The grid's own scroller, for a surface that has to move two grids as one: the two panes of a
   * comparison hold the same rows in the same order, and a pane scrolled on its own would put row
   * *n* of one beside row *m* of the other, which is the one thing that makes a diff unreadable.
   *
   * It is the *element* rather than a scroll position, because who mirrors whom — and how a loop
   * between two of them is broken — is the call site's business and not the grid's. Pass a stable
   * ref: it is attached with the element and would otherwise detach on every render.
   */
  viewportRef?: RefObject<HTMLDivElement | null> | undefined
  /**
   * Draws the grid to be *scanned* rather than worked in: shorter rows, a one-line header, a
   * narrower gutter. It is a surface that has to fit several grids in one pane that asks for it —
   * a comparison is two of them and a script — and nothing is left out to get it.
   */
  dense?: boolean
  /**
   * Draws the grid as a **strip that follows another**: no row gutter and no scrollbars of its own,
   * because the grid beside it already carries both and a second set would only repeat them.
   *
   * It exists for one thing — the column of ticks between a comparison's two panes, which is a pane
   * of its own precisely so that scrolling either side sideways cannot slide it away. Such a strip
   * is never read alone, so it is moved through `viewportRef` like the panes it sits between.
   */
  strip?: boolean
}

/** The head strip has nothing to say for a grid with none of the things it carries. */
const hasHeadStrip = (props: Pick<ResultGridProps, 'onExport' | 'filtering' | 'cellPanel'>) =>
  props.onExport !== undefined || props.filtering !== undefined || props.cellPanel !== undefined

export function ResultGrid({
  result,
  selection,
  onSelectionChange,
  onEditCell,
  isDirty = NOTHING_DIRTY,
  isDeleted = NOTHING_DELETED,
  markRow = NOTHING_MARKED,
  markCell = NO_CELL_MARKED,
  sort = NO_SORT,
  onSortChange,
  onActivateRow,
  columnMenuItems,
  onMoveColumn,
  onExport,
  headStrip = true,
  onEditInPanel,
  cellPanel,
  onDeleteRows,
  onRefresh,
  filtering,
  formatValue,
  viewportRef,
  dense = false,
  strip = false,
}: ResultGridProps) {
  const density = dense ? DENSE : ROOMY
  const size = strip ? { ...density, gutter: 0 } : density

  const scrollRef = useRef<HTMLDivElement>(null)
  const bodyRef = useRef<HTMLDivElement>(null)
  const drag = useRef<Drag | null>(null)
  const lastHit = useRef<Hit | null>(null)
  const pointer = useRef({ x: 0, y: 0 })
  const frame = useRef(0)
  const [resized, setResized] = useState<Record<string, number>>({})
  const [ownSelection, setOwnSelection] = useState<Selection | null>(null)
  const [editing, setEditing] = useState<CellRef | null>(null)
  /** The cell the last right-click landed on, which the menu names — see `onCellContextMenu`. */
  const [menuCell, setMenuCell] = useState<CellRef | null>(null)
  /** The field a header's **Add Filter** asked for, which the panel takes into its draft. */
  const [filterAsk, setFilterAsk] = useState<FilterAsk | null>(null)
  /**
   * The header being carried, if one is. It is the grid's rather than the header row's because the
   * column it marks runs the whole height of the grid: what the reader has hold of is a column, and
   * a mark that stopped at the header would say only which *header* they had.
   */
  const [columnDrag, setColumnDrag] = useState<ColumnDrag | null>(null)

  /**
   * The cell open in the value viewer, or null. It is the grid's own rather than a call site's
   * because *every* grid has cells worth looking at whole — a query result's JSON, a history
   * entry's statement — and only the two writable surfaces have a cell-editor panel.
   */
  const [viewing, setViewing] = useState<CellRef | null>(null)

  // The scroller is the grid's own and a call site's at once, where one was asked for. A callback
  // rather than two `ref` attributes, because an element takes one — and a memoized one, because
  // an inline callback ref detaches and reattaches the scroller on every render.
  const attachViewport = useCallback(
    (node: HTMLDivElement | null) => {
      scrollRef.current = node
      if (viewportRef) viewportRef.current = node
    },
    [viewportRef],
  )

  const current = selection === undefined ? ownSelection : selection

  const select = (next: Selection) => {
    if (selection === undefined) setOwnSelection(next)
    onSelectionChange?.(next)
  }

  // The reader's own width first, then the column's own, then the grid's — so a drag outlives the
  // width a call site asked to open at, and neither of them outlives the other's turn.
  const widths = result.columns.map(
    (column) => resized[column.name] ?? column.width ?? DEFAULT_COLUMN_WIDTH,
  )
  const lastColumn = result.columns.length - 1

  const virtualizer = useVirtualizer({
    count: result.rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => size.row,
    overscan: 16,
  })

  useEffect(() => () => cancelAnimationFrame(frame.current), [])

  // Rows shift under the open input whenever a row is added or dropped, so the editor closes with
  // them. The dependency is the *count* and not the row array: a call site rebuilds that array on
  // every render of its own, and closing the editor there would drop what was being typed.
  useEffect(() => setEditing(null), [result.rows.length])

  // Closing an editor hands the keyboard back to the grid, so `Escape` lands the reader on the
  // cell they were editing with the arrows live again. Only ever after one was open, and only
  // where the keyboard is still the grid's: an editor closed by a click on something else — a
  // toolbar box, the SQL editor — must leave the focus where that click put it.
  const wasEditing = useRef(false)
  useEffect(() => {
    if (editing !== null) {
      wasEditing.current = true
      return
    }
    if (!wasEditing.current) return
    wasEditing.current = false

    const active = document.activeElement
    if (active === null || active === document.body || scrollRef.current?.contains(active)) {
      scrollRef.current?.focus()
    }
  }, [editing])

  /**
   * Which panels the splitter holds, named for it. A layout is remembered per *combination*, so the
   * height the reader gave the filter panel is theirs the next time they open it, and closing it
   * does not teach the rows a new size.
   */
  const panelIds = useMemo(
    () => (filtering?.shown ? ['filter', 'rows'] : ['rows']),
    [filtering?.shown],
  )

  const totalWidth = size.gutter + widths.reduce((sum, w) => sum + w, 0)
  const bounds = current?.ranges.map(boundsOf) ?? []
  const focus = focusOf(current)

  const setWidth = (name: string, width: number) =>
    setResized((previous) => ({ ...previous, [name]: Math.max(MIN_COLUMN_WIDTH, width) }))

  const fitColumn = (index: number) => {
    const column = result.columns[index]
    if (!column) return

    const font = gridFont()
    const cells = result.rows.slice(0, FIT_SAMPLE).map((row) => cellPreview(row[index] ?? null))
    const key = column.primaryKey === true ? KEY_ICON_WIDTH : 0
    const content = Math.max(
      widestText(cells, font),
      widestText([column.name], headerFont()) + key + MENU_WIDTH,
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

    // A header sorts by its own column and nothing else: one click means one key, and the panel
    // is where a second is added under the first.
    const sorting: MenuItem[] = onSortChange
      ? [
          {
            label: 'Sort Ascending',
            icon: ICON.sortAscending,
            onSelect: () => onSortChange([{ column: column.name, descending: false }]),
          },
          {
            label: 'Sort Descending',
            icon: ICON.sortDescending,
            onSelect: () => onSortChange([{ column: column.name, descending: true }]),
          },
          {
            label: 'Remove All Sorts',
            icon: ICON.sortNone,
            disabled: sort.length === 0,
            onSelect: () => onSortChange([]),
          },
        ]
      : []

    // A header knows the field and not the value, so what it adds is unspelled: it opens the panel
    // and lands in its draft, where the value is picked and Apply is what narrows the rows.
    const narrowing: MenuItem[] = filtering
      ? [
          {
            label: 'Add Filter',
            icon: ICON.filter,
            onSelect: () => {
              filtering.onShownChange(true)
              setFilterAsk({ column: column.name })
            },
          },
        ]
      : []

    return [
      ...sorting,
      ...narrowing,
      {
        label: 'Copy column',
        icon: ICON.copy,
        separatorBefore: sorting.length + narrowing.length > 0,
        disabled: result.rows.length === 0,
        onSelect: () =>
          void copyText(toTSV(oneRange(columnRange(index, result.rows.length)), result)),
      },
      { label: 'Copy field name', icon: ICON.copy, onSelect: () => void copyText(column.name) },
      ...(onEditCell
        ? [
            {
              label: 'Paste',
              icon: ICON.paste,
              disabled: result.rows.length === 0,
              onSelect: () => void paste({ row: 0, column: index }),
            },
          ]
        : []),
      ...(columnMenuItems?.(column) ?? []),
    ]
  }

  /**
   * A right-click raises the grid's own menu, never the WebView's, and names the cell under the
   * pointer. A press **outside** the selection replaces it with that one cell; one **inside** it
   * leaves every rectangle standing, so a range the reader picked can be acted on whole. A press
   * in the row gutter takes the whole row, exactly as a press there does with the left button.
   */
  const onCellContextMenu = (event: ReactMouseEvent<HTMLDivElement>) => {
    if (result.rows.length === 0) return

    const ref = cellAt(event.clientX, event.clientY, false)
    setMenuCell(ref)

    if (within(bounds, ref.row, ref.column)) return
    select(
      inGutter(event.target)
        ? oneRange(rowRange(ref.row, result.columns.length))
        : oneRange({ anchor: ref, focus: ref }),
    )
  }

  /**
   * The cell menu is built from the cell the right-click named, falling back to the focus so the
   * menu exists before the first one — an item list that was empty would leave no trigger for
   * Radix to open, and the first right-click would raise the WebView's menu instead of this.
   */
  const cellMenu = (): MenuItem[] => {
    const at = menuCell ?? focus ?? { row: 0, column: 0 }

    return cellMenuItems({
      at,
      data: result,
      selection: current ?? oneRange({ anchor: at, focus: at }),
      sort,
      onEditCell,
      onEditInPanel,
      onDeleteRows,
      onSortChange,
      onExport,
      onRefresh,
      filtering,
      onPaste: (origin) => void paste(origin),
      onViewValue: setViewing,
    })
  }

  /** Reads a cell straight off the pointer, so a drag needs no per-cell handlers. */
  const cellAt = (clientX: number, clientY: number, wholeRow: boolean): CellRef => {
    const rect = bodyRef.current?.getBoundingClientRect()
    const row = clamp(Math.floor((clientY - (rect?.top ?? 0)) / size.row), result.rows.length - 1)
    const column = wholeRow
      ? lastColumn
      : columnAt(widths, clientX - (rect?.left ?? 0) - size.gutter)

    return { row, column }
  }

  /**
   * Brings a cell inside the viewport, clear of the sticky header and the row gutter, and leaves
   * the scroll where it is when the cell is already there.
   */
  const reveal = (ref: CellRef) => {
    const scroller = scrollRef.current
    if (!scroller) return

    const top = ref.row * size.row
    const left = edgeOf(widths, ref.column, size.gutter)

    scroller.scrollTo({
      top: keepWithin(
        scroller.scrollTop,
        top + size.row - scroller.clientHeight,
        top - size.header,
      ),
      left: keepWithin(
        scroller.scrollLeft,
        left + (widths[ref.column] ?? 0) - scroller.clientWidth,
        left - size.gutter,
      ),
    })
  }

  /** Walks the focus one cell, or stretches the live rectangle to it while `Shift` is held. */
  const move = (step: CellRef, extending: boolean) => {
    if (result.rows.length === 0) return

    const next = focus
      ? {
          row: clamp(focus.row + step.row, result.rows.length - 1),
          column: clamp(focus.column + step.column, lastColumn),
        }
      : { row: 0, column: 0 }
    const live = current?.ranges.at(-1)

    select(
      extending && live
        ? { ranges: [...(current?.ranges ?? []).slice(0, -1), { anchor: live.anchor, focus: next }] }
        : oneRange({ anchor: next, focus: next }),
    )
    reveal(next)
  }

  const extend = (clientX: number, clientY: number) => {
    const state = drag.current
    if (!state) return

    const next = cellAt(clientX, clientY, state.wholeRow)
    if (next.row === state.focus.row && next.column === state.focus.column) return

    state.focus = next
    if (lastHit.current) lastHit.current.moved = true
    select({ ranges: [...state.kept, { anchor: state.anchor, focus: next }] })
  }

  const autoScroll = () => {
    const scroller = scrollRef.current
    if (!drag.current || !scroller) return

    const rect = scroller.getBoundingClientRect()
    const { x, y } = pointer.current
    const left = edgePush(x, rect.left, rect.right)
    const top = edgePush(y, rect.top + size.header, rect.bottom)

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
    lastHit.current = {
      ref: hit,
      wholeRow,
      extending: event.shiftKey || event.ctrlKey || event.metaKey,
      moved: false,
    }
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

  /**
   * Flips a `check` or a `key` cell, and says whether the cell was one — a toggle is written
   * rather than opened, so nothing that opens an editor may also run on it.
   */
  const toggle = (ref: CellRef): boolean => {
    const editor = result.columns[ref.column]?.editor
    if (!onEditCell || !isToggle(editor)) return false

    onEditCell(ref, checked(result.rows[ref.row]?.[ref.column] ?? null) ? '' : CHECK_ON)
    return true
  }

  /** Whether this column takes an edit at all — see `ResultColumn.readOnly`. */
  const writable = (column: number) => result.columns[column]?.readOnly !== true

  /**
   * **One click opens the cell it lands on**, which is the whole of what a click does beyond
   * selecting: a toggle is flipped, anything else opens its editor. Read off the press — pointer
   * capture retargets the click that follows one — and never from a drag, a row gutter, or a
   * press that was reaching for a range.
   */
  const onClick = () => {
    const hit = lastHit.current
    if (!onEditCell || !hit || hit.wholeRow || hit.moved || hit.extending) return
    if (!writable(hit.ref.column)) return
    if (!toggle(hit.ref)) setEditing(hit.ref)
  }

  /** Read off the press for the reason a click is: the row under the pointer moved with it. */
  const onDoubleClick = () => {
    const hit = lastHit.current
    if (hit && !hit.moved) onActivateRow?.(hit.ref.row)
  }

  const onKeyDown = (event: ReactKeyboardEvent<HTMLDivElement>) => {
    // An open editor owns the keyboard: its own arrows walk its text, or its list.
    if (editing) return

    const step = ARROWS[event.key]
    if (step) {
      event.preventDefault()
      move(step, event.shiftKey)
      return
    }
    if (!current) return

    if (event.key === 'Enter' && onEditCell && focus && writable(focus.column)) {
      event.preventDefault()
      if (!toggle(focus)) setEditing(focus)
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

  /** The header and the rows under it: the whole of the grid but whatever panel sits over it. */
  const rows = (
    <div
      ref={attachViewport}
      tabIndex={0}
      onKeyDown={onKeyDown}
      className={cn(
        'overflow-auto bg-canvas',
        strip ? 'scroll-none' : 'scroll-thin',
        // It fills the panel the splitter gave it where there is one, and takes what the column
        // leaves where there is not.
        filtering ? 'h-full' : 'min-h-0 flex-1',
      )}
    >
      <div className="relative w-full" style={{ minWidth: totalWidth }}>
        <HeaderRow
          columns={result.columns}
          widths={widths}
          bounds={bounds}
          sort={sort}
          onResize={setWidth}
          onFit={fitColumn}
          onSelectColumn={selectColumn}
          onMoveColumn={onMoveColumn}
          drag={columnDrag}
          onDragChange={setColumnDrag}
          menuFor={columnMenu}
          size={size}
          dense={dense}
          strip={strip}
        />

        {/* What is being carried, and where it would land — both drawn down the whole grid
            rather than across the header alone, because a column is what is being moved and
            the rows under it are what the reader is reading. */}
        {columnDrag ? (
          <span
            className="pointer-events-none absolute top-0 bottom-0 z-3 border-[3px] border-fg"
            style={{
              left: edgeOf(widths, columnDrag.from, size.gutter),
              width: widths[columnDrag.from] ?? DEFAULT_COLUMN_WIDTH,
            }}
          />
        ) : null}
        {columnDrag && moves(columnDrag) ? (
          <span
            className="pointer-events-none absolute top-0 bottom-0 z-3 w-0.5 bg-accent"
            style={{ left: edgeOf(widths, columnDrag.to, size.gutter) }}
          />
        ) : null}

        <ContextMenu items={cellMenu()}>
          <div
            ref={bodyRef}
            onPointerDown={onPointerDown}
            onPointerMove={onPointerMove}
            onPointerUp={endDrag}
            onLostPointerCapture={endDrag}
            onClick={onClick}
            onDoubleClick={onDoubleClick}
            onContextMenu={onCellContextMenu}
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
              // What a comparison says about the row, and nothing for one the two sides agree
              // about. Deletion wins over it, being the more consequential fact — and no surface
              // is ever both a comparison and writable, so the two never actually meet.
              const mark = removed ? undefined : markRow(item.index)

              return (
                <div
                  key={item.key}
                  className={cn(
                    'absolute top-0 left-0 flex w-full border-b border-line/60',
                    removed
                      ? 'bg-grid-row-deleted line-through'
                      : banded
                        ? 'bg-grid-row-selected'
                        : mark
                          ? DIFF_ROW[mark]
                          : 'hover:bg-hover',
                  )}
                  style={{ height: size.row, transform: `translateY(${item.start}px)` }}
                >
                  {size.gutter > 0 ? (
                    <div
                      data-gutter=""
                      className={cn(
                        'sticky left-0 z-1 flex shrink-0 items-center justify-end border-r border-line pr-2 font-mono text-2xs',
                        removed
                          ? 'bg-grid-row-deleted text-danger'
                          : touched
                            ? 'bg-grid-row-selected text-fg shadow-[inset_2px_0_0_var(--accent)]'
                            : mark
                              ? DIFF_GUTTER[mark]
                              : 'bg-panel text-faint',
                      )}
                      style={{ width: size.gutter }}
                    >
                      {item.index + 1}
                    </div>
                  ) : null}

                  {result.columns.map((column, i) => {
                    const width = widths[i] ?? DEFAULT_COLUMN_WIDTH

                    if (onEditCell && editing?.row === item.index && editing.column === i) {
                      const commit = (value: Cell) => {
                        onEditCell({ row: item.index, column: i }, value)
                        setEditing(null)
                      }
                      const cancel = () => setEditing(null)

                      return column.editor?.kind === 'choice' ? (
                        <CellChoice
                          key={i}
                          value={row[i] ?? null}
                          options={column.editor.options}
                          width={width}
                          onCommit={commit}
                          onCancel={cancel}
                        />
                      ) : (
                        <CellInput
                          key={i}
                          value={row[i] ?? null}
                          picker={temporalPicker(column.type)}
                          width={width}
                          onCommit={commit}
                          onCancel={cancel}
                        />
                      )
                    }

                    return (
                      <CellView
                        key={i}
                        value={row[i] ?? null}
                        editor={column.editor}
                        numeric={isNumeric(column.type)}
                        width={width}
                        inRange={within(bounds, item.index, i)}
                        focused={focus?.row === item.index && focus.column === i}
                        edited={isDirty(item.index, i)}
                        differs={mark !== undefined && markCell(item.index, i)}
                        row={item.index}
                        outline={column.outline}
                      />
                    )
                  })}
                </div>
              )
            })}
          </div>
        </ContextMenu>
      </div>
    </div>
  )

  return (
    <div className="flex h-full min-h-0 flex-col">
      {headStrip && hasHeadStrip({ onExport, filtering, cellPanel }) ? (
        <HeadStrip
          filtering={filtering}
          sort={sort}
          cellPanel={cellPanel}
          onExport={onExport}
        />
      ) : null}

      {/* A grid that can be narrowed carries the splitter, because the panel that narrows it is
          dragged rather than fixed: a reader who opened it still says how much of the pane the
          rows keep, and the handle is the only divider between the two. One that cannot be
          narrowed is the rows alone, and has no panel to be given room. */}
      {filtering ? (
        <Split
          id="vibe-db.grid.filter"
          orientation="vertical"
          panelIds={panelIds}
          className="min-h-0 flex-1"
        >
          {filtering.shown ? (
            <>
              <SplitPanel
                id="filter"
                defaultSize={FILTER_HEIGHT}
                minSize={FILTER_MIN_HEIGHT}
                maxSize="70%"
              >
                <FilterSortPanel
                  columns={result.columns}
                  filtering={filtering}
                  sort={sort}
                  onSortChange={onSortChange}
                  ask={filterAsk}
                  onAskTaken={() => setFilterAsk(null)}
                />
              </SplitPanel>

              <SplitHandle orientation="vertical" />
            </>
          ) : null}

          <SplitPanel id="rows" className="min-h-0">
            {rows}
          </SplitPanel>
        </Split>
      ) : (
        rows
      )}

      <ValueDialog
        at={viewing}
        data={result}
        formatValue={formatValue}
        onClose={() => setViewing(null)}
      />
    </div>
  )
}

/**
 * The strip over the header, where the grid is acted on whole rather than row by row: **Filter &
 * Sort** on the left, where a reader looks for what narrows the table, and against the right
 * **Cell Editor** and **Export**, where they look for what to do with it. It sits outside the
 * scroller, so it stays where it is while the columns scroll under it, and it carries the header's
 * own surface, the two together reading as the top of the table. A surface with a toolbar of its
 * own carries the same buttons there instead, and the grid then draws no strip at all.
 */
function HeadStrip({
  filtering,
  sort,
  cellPanel,
  onExport,
}: {
  filtering: GridFiltering | undefined
  sort: readonly SortKey[]
  cellPanel: { shown: boolean; onToggle: () => void } | undefined
  onExport: ((format: ExportFormat) => void) | undefined
}) {
  return (
    <div
      role="toolbar"
      className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5"
    >
      {filtering ? (
        <FilterSortButton
          shown={filtering.shown}
          count={countConditions(filtering.criteria) + sort.length}
          onToggle={() => filtering.onShownChange(!filtering.shown)}
        />
      ) : null}

      <div className="min-w-2 flex-1" />

      {cellPanel ? (
        <CellEditorButton shown={cellPanel.shown} onToggle={cellPanel.onToggle} />
      ) : null}
      {onExport ? <ExportMenu onExport={onExport} /> : null}
    </div>
  )
}

const typeLabel = (column: ResultColumn) => column.fullType || column.type

const inGutter = (target: EventTarget) =>
  target instanceof Element && target.closest('[data-gutter]') !== null

const clamp = (value: number, highest: number) => Math.min(Math.max(value, 0), Math.max(highest, 0))

/** Moves a scroll offset the least it can to satisfy both edges, and not at all where it already does. */
const keepWithin = (value: number, lowest: number, highest: number) =>
  Math.min(Math.max(value, lowest), highest)

/** Where a column's left edge sits across the grid's whole width, the row gutter included. */
function edgeOf(widths: number[], index: number, gutter: number): number {
  return gutter + widths.slice(0, index).reduce((sum, width) => sum + width, 0)
}

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
  sort: readonly SortKey[]
  onResize: (name: string, width: number) => void
  onFit: (index: number) => void
  onSelectColumn: (index: number) => void
  onMoveColumn: ((from: number, to: number) => void) | undefined
  /** The drag in progress, which the grid holds because the mark it draws is a column's. */
  drag: ColumnDrag | null
  onDragChange: (drag: ColumnDrag | null) => void
  menuFor: (index: number) => MenuItem[]
  /** The geometry the grid picked, which the header has to draw itself and its gutter at. */
  size: Density
  /** Whether the name and its type share one line — see `ResultGridProps.dense`. */
  dense: boolean
  /**
   * A strip's header carries the name and nothing else — see `ResultGridProps.strip`. Its width is
   * the frame's and not the reader's to drag: a column wider than the strip around it could not be
   * scrolled back into view, the strip having no bar to scroll it with.
   */
  strip: boolean
}

/** A header being carried, and the boundary between two columns it would be dropped on. */
interface ColumnDrag {
  from: number
  to: number
}

/** Whether a drop would land the column anywhere but back where it came from. */
const moves = (drag: ColumnDrag) => drag.to !== drag.from && drag.to !== drag.from + 1

function HeaderRow({
  columns,
  widths,
  bounds,
  sort,
  onResize,
  onFit,
  onSelectColumn,
  onMoveColumn,
  drag,
  onDragChange,
  menuFor,
  size,
  dense,
  strip,
}: HeaderRowProps) {
  const bar = useRef<HTMLDivElement>(null)

  /**
   * The boundary the pointer is nearest, counted like a caret between characters: past a column's
   * half-way line is the boundary *after* it, which is how a drop reads. The row scrolls with the
   * columns, so it is measured from the row's own left edge exactly as a cell is.
   */
  const boundaryAt = (clientX: number): number => {
    const x = clientX - (bar.current?.getBoundingClientRect().left ?? 0) - size.gutter

    let edge = 0
    for (let i = 0; i < widths.length; i++) {
      const width = widths[i] ?? 0
      if (x < edge + width / 2) return i
      edge += width
    }
    return widths.length
  }

  const startDrag = (event: ReactPointerEvent<HTMLDivElement>, index: number) => {
    // The resize handle stops its own press; a button in the header is pressed, never gripped.
    if (!onMoveColumn || event.button !== 0) return
    if (event.target instanceof Element && event.target.closest('button')) return

    event.currentTarget.setPointerCapture(event.pointerId)
    onDragChange({ from: index, to: index })
  }

  const carry = (event: ReactPointerEvent<HTMLDivElement>) => {
    if (!drag) return

    const to = boundaryAt(event.clientX)
    if (to !== drag.to) onDragChange({ ...drag, to })
  }

  const drop = () => {
    if (!drag) return

    onDragChange(null)
    if (moves(drag)) onMoveColumn?.(drag.from, drag.to)
  }

  return (
    <div ref={bar} className="sticky top-0 z-2 flex border-b border-line-strong bg-bar">
      {size.gutter > 0 ? (
        <div
          className="sticky left-0 z-1 shrink-0 border-r border-line bg-bar"
          style={{ width: size.gutter, height: size.header }}
        />
      ) : null}

      {columns.map((column, i) => {
        const active = columnTouched(bounds, i)
        // Which key this column is, so a header under the *second* key says so rather than
        // claiming the order for itself.
        const place = sort.findIndex((key) => key.column === column.name)
        const sorted = sort[place]

        return (
          <ContextMenu key={i} items={menuFor(i)}>
            <div
              title={`${column.name} — ${typeLabel(column)}`}
              onContextMenu={() => onSelectColumn(i)}
              onPointerDown={(event) => startDrag(event, i)}
              onPointerMove={carry}
              onPointerUp={drop}
              onLostPointerCapture={drop}
              className={cn(
                'relative flex shrink-0 flex-col justify-center border-r border-line pl-2',
                strip ? 'pr-2' : 'pr-7',
                active ? 'bg-grid-cell-selected' : 'bg-bar',
                onMoveColumn && 'cursor-grab',
                drag?.from === i && 'opacity-50',
              )}
              style={{ width: widths[i] ?? DEFAULT_COLUMN_WIDTH, height: size.header }}
            >
              <span className="flex items-center gap-1">
                {/* The same lock the tree draws on a key field, in the same gold: which columns
                    name a row is the first thing a reader asks of a table they did not write. */}
                {column.primaryKey === true ? (
                  <KeyRound size={11} className="shrink-0 text-grid-key" />
                ) : null}
                {/* A dense header puts the type-kind glyph beside the name instead of on a line
                    of its own: the colour is what the icon is read for, and it costs no height. */}
                {dense ? <ColumnTypeIcon type={column.type} /> : null}
                {/* The UI font, where a cell is mono: a name read as a label cannot be mistaken
                    for the data under it. */}
                <span
                  className={cn(
                    'min-w-0 truncate font-bold tracking-wide text-fg',
                    dense ? 'text-xs' : 'text-base',
                  )}
                >
                  {column.name}
                </span>
                {sorted ? (
                  <SortMark
                    descending={sorted.descending}
                    place={sort.length > 1 ? place + 1 : 0}
                  />
                ) : null}
              </span>

              {/* The type line is the first thing a dense header gives up: it is the one fact of
                  the two that the *cells* underneath repeat, and its icon has moved up beside
                  the name. The whole of it is still in the header's own tooltip. */}
              {dense ? null : (
                <span className="flex items-center gap-1 text-2xs text-muted">
                  <ColumnTypeIcon type={column.type} />
                  <span className="truncate font-mono">{typeLabel(column)}</span>
                </span>
              )}

              {strip ? null : (
                <>
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
                </>
              )}
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

/** Which way the column runs, and — where a second key is in force — which key this one is. */
function SortMark({ descending, place }: { descending: boolean; place: number }) {
  const Icon = descending ? ArrowDownNarrowWide : ArrowUpNarrowWide

  return (
    <span className="flex shrink-0 items-center text-accent">
      <Icon size={12} />
      {place > 0 ? <span className="font-mono text-2xs">{place}</span> : null}
    </span>
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
  editor: ColumnEditor | undefined
  numeric: boolean
  width: number
  inRange: boolean
  focused: boolean
  edited: boolean
  /** Whether a comparison found this cell's two sides different — see `markCell`. */
  differs: boolean
  /** Which row this is, which only an outlined cell needs: everything else is in `value`. */
  row: number
  /** Set where this column draws its cells as the rows of a tree — see `GridOutline`. */
  outline: GridOutline | undefined
}

function CellView({
  value,
  editor,
  numeric,
  width,
  inRange,
  focused,
  edited,
  differs,
  row,
  outline,
}: CellViewProps) {
  const tone = focused
    ? 'text-on-accent'
    : value === null
      ? 'text-null'
      : edited
        ? 'text-warn'
        : 'text-fg'

  return (
    <div
      className={cn(
        'flex shrink-0 items-center gap-1 overflow-hidden border-r border-line/60 px-2',
        'data-font whitespace-nowrap',
        numeric && 'justify-end',
        isToggle(editor) && 'justify-center',
        // The row's own wash already says the row differs; this deepens the cells that are why.
        focused ? 'bg-accent' : inRange ? 'bg-grid-cell-selected' : differs && 'bg-grid-diff-cell',
        edited && 'shadow-[inset_0_0_0_1px_var(--warn)]',
      )}
      style={{ width }}
    >
      {outline ? <OutlineMark outline={outline} row={row} focused={focused} /> : null}

      {editor?.kind === 'check' ? (
        <CheckMark on={checked(value)} focused={focused} />
      ) : editor?.kind === 'key' ? (
        <KeyMark position={value} tone={tone} />
      ) : (
        <>
          <span className={cn('min-w-0 truncate', tone)}>{cellPreview(value)}</span>
          {editor?.kind === 'choice' ? (
            <ChevronDown
              size={12}
              className={cn('ml-auto shrink-0', focused ? 'text-on-accent' : 'text-faint')}
            />
          ) : null}
        </>
      )}
    </div>
  )
}

/**
 * The indent, the caret and the glyph that turn a column of names into a tree.
 *
 * The caret **stops its own press**, for the reason the resize handle does: the grid reads a drag
 * off the press on its body, and a caret that let one through would start selecting cells every
 * time a branch was opened. A leaf keeps the caret's width so every name on one level starts on
 * one line.
 */
function OutlineMark({
  outline,
  row,
  focused,
}: {
  outline: GridOutline
  row: number
  focused: boolean
}) {
  const open = outline.open(row)

  return (
    <>
      <span aria-hidden className="shrink-0" style={{ width: outline.depth(row) * OUTLINE_INDENT }} />

      {open === undefined ? (
        <span aria-hidden className="w-3.5 shrink-0" />
      ) : (
        <button
          type="button"
          aria-label={open ? 'Collapse' : 'Expand'}
          onPointerDown={(event) => event.stopPropagation()}
          onClick={(event) => {
            event.stopPropagation()
            outline.onToggle(row)
          }}
          className={cn(
            'grid h-4 w-3.5 shrink-0 place-items-center',
            focused ? 'text-on-accent' : 'text-muted hover:text-fg',
          )}
        >
          {open ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
        </button>
      )}

      {outline.icon ? <span className="shrink-0">{outline.icon(row)}</span> : null}
    </>
  )
}

/** A yes/no, drawn as the box it is clicked like. Empty rather than `(Null)`: a no is a no. */
function CheckMark({ on, focused }: { on: boolean; focused: boolean }) {
  const Glyph = on ? SquareCheck : Square

  return (
    <Glyph
      size={14}
      className={cn('shrink-0', focused ? 'text-on-accent' : on ? 'text-accent' : 'text-faint')}
    />
  )
}

/** A field's place in the primary key: the lock, then which of its columns this one is. */
function KeyMark({ position, tone }: { position: Cell; tone: string }) {
  if (!checked(position)) return null

  return (
    <>
      <KeyRound size={13} className="shrink-0 text-grid-key" />
      <span className={cn('text-2xs', tone)}>{position}</span>
    </>
  )
}

interface CellInputProps {
  value: Cell
  /** The picker this column's type takes, where it takes one: the `⋮` beside the box opens it. */
  picker: TemporalPicker | null
  width: number
  onCommit: (value: Cell) => void
  onCancel: () => void
}

/** Leaving a NULL cell empty commits NULL again, so opening the editor never rewrites it. */
function CellInput({ value, picker, width, onCommit, onCancel }: CellInputProps) {
  const box = useRef<HTMLDivElement>(null)
  const text = useRef<HTMLInputElement>(null)
  const [draft, setDraft] = useState(value ?? '')
  const [picking, setPicking] = useState(false)

  const commit = () => onCommit(draft === '' && value === null ? null : draft)

  const closePicker = () => {
    setPicking(false)
    text.current?.focus()
  }

  return (
    <div
      ref={box}
      // The editor is not a cell: a press inside it must not start a selection under it.
      onPointerDown={(event) => event.stopPropagation()}
      style={{ width }}
      className="relative flex shrink-0 items-center border-r border-line/60 bg-canvas shadow-[inset_0_0_0_1px_var(--accent)]"
    >
      <input
        ref={text}
        autoFocus
        value={draft}
        spellCheck={false}
        onChange={(event) => setDraft(event.target.value)}
        // The picker is part of the editor, so the blur that opening it costs is not a commit.
        onBlur={() => {
          if (!picking) commit()
        }}
        onKeyDown={(event) => {
          if (event.key === 'Enter') commit()
          else if (event.key === 'Escape') onCancel()
          else return

          event.preventDefault()
          event.stopPropagation()
        }}
        className="data-font h-full min-w-0 flex-1 bg-transparent px-2 text-fg outline-none"
      />

      {picker ? (
        <IconButton
          label="Pick a value"
          icon={<EllipsisVertical size={14} />}
          // Opening the picker must not blur the box, or the commit that blur runs would beat it.
          onMouseDown={(event) => event.preventDefault()}
          // Closing from the button hands the keyboard back to the box, as `Escape` does.
          onClick={() => (picking ? closePicker() : setPicking(true))}
          // Its own border and surface, or it reads as part of the value rather than a control.
          className={cn('mr-0.5 border border-line-strong', picking ? 'bg-hover' : 'bg-panel')}
        />
      ) : null}

      {picker && picking ? (
        <CellPicker
          picker={picker}
          value={draft}
          anchor={box}
          onChange={setDraft}
          onCommit={commit}
          onCancel={closePicker}
        />
      ) : null}
    </div>
  )
}
