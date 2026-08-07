import type { ReactNode } from 'react'

import type { Cell, TableColumn } from '@/bridge/tables'

import { isNumeric } from './columnType'

export type { Cell }

export const NULL_TEXT = '(Null)'

/** What a toggled cell holds, so what the grid writes and what a copy reads cannot disagree. */
export const CHECK_ON = 'YES'

/**
 * How a column's cells are drawn and changed, where the grid is writable. Text unless a call site
 * names one: a `choice` opens a searchable list of the values the column accepts, while `check`
 * and `key` are flipped by a single click, a yes/no having nothing to type into it. A `key` cell
 * shows its own value beside the lock, which is the field's place in the key and the call site's
 * to count.
 */
export type ColumnEditor =
  | { kind: 'choice'; options: readonly string[] }
  | { kind: 'check' }
  | { kind: 'key' }

/**
 * How a column's cells are drawn as the rows of a **tree**: indented by how deep the row sits, with
 * a caret on a row that holds others and a glyph saying what it is.
 *
 * It is a *column* and not a mode of the grid, because that is what it actually is — one column
 * carries the hierarchy and every other column beside it stays a plain fact about the row. The rows
 * are still a flat list, in the order they read, and the call site is what flattens the tree into
 * one: the grid neither holds a shape nor decides what is open.
 *
 * A grid with rows and not branches was the reason `PlanNode.Depth` is drawn flat; this is what
 * that would need.
 */
export interface GridOutline {
  /** How deep this row sits. Zero is the top, and every row of a flat list.  */
  depth: (row: number) => number
  /** Whether the row holds others: `undefined` for a leaf, and open or shut for a branch. */
  open: (row: number) => boolean | undefined
  onToggle: (row: number) => void
  /** What the row *is*, drawn ahead of its name — a folder, a field, an index. */
  icon?: (row: number) => ReactNode
  /**
   * A number about the row rather than a value of it — how many things a branch holds — drawn after
   * its name as the object tree draws one: a pill, in muted mono, clear of the name's own colour and
   * weight. It is **chrome and not the cell**, which is the whole reason it exists: a count spelled
   * into the cell's text is a count that a copy writes out, that a fit measures and that an edit
   * would hand back as part of the name.
   */
  badge?: (row: number) => string | undefined
}

export interface ResultColumn {
  name: string
  /** The bare type, which classifies the column. `fullType` carries its length and modifiers. */
  type: string
  fullType: string
  editor?: ColumnEditor | undefined
  /**
   * Whether the column is part of the primary key, which the header marks with a lock. A result
   * that is not a stored table's rows carries none — an expression has no key to be part of.
   *
   * It is the one stored fact the header may draw. **Nullability is not**: a driver that cannot
   * say reports `false`, which is indistinguishable from a column the server really will not take
   * NULL for, and a grid cannot tell the two apart. The designer reads the catalogue and can.
   */
  primaryKey?: boolean | undefined
  /**
   * Whether this column refuses an edit on an otherwise writable grid. It exists for the one
   * arrangement a single `onEditCell` cannot express: a grid whose rows are *read* and which
   * carries one column that is acted on — a comparison's **Take** tick beside four columns of
   * facts. Without it, passing `onEditCell` for the tick would open a text editor on every fact
   * beside it, and none of them is a thing a reader may type over.
   */
  readOnly?: boolean | undefined
  /**
   * The width this column opens at, where the grid's own is wrong for it — a column of **ticks** is
   * not as wide as a column of values, and a tree's name column is wider than one. It is only the
   * width it *opens* at: a drag or a fit replaces it, and the reader's width always wins.
   */
  width?: number | undefined
  /** Draws this column's cells as the rows of a tree — see `GridOutline`. */
  outline?: GridOutline | undefined
}

/**
 * What a comparison says about one row of a grid drawing one side of it. The first three are
 * `bridge/compare`'s own statuses; `absent` is the fourth thing a *pane* has to draw and the diff
 * has no word for — the blank standing opposite a row this side simply has not got, which is
 * neither a value nor an empty one. A row the two sides agree about carries no mark at all.
 *
 * It is the grid's own type rather than the bridge's because the grid must not learn what a
 * comparison is: it is handed a word per row and paints it.
 */
export type DiffMark = 'added' | 'removed' | 'changed' | 'absent'

/** A toggle cell is on when it holds anything at all, a key cell holding its position instead. */
export function checked(value: Cell): boolean {
  return value !== null && value !== ''
}

/** Whether the column is flipped rather than opened, which is what a click on one does. */
export function isToggle(editor: ColumnEditor | undefined): boolean {
  return editor?.kind === 'check' || editor?.kind === 'key'
}

/**
 * One column a grid is sorted by, named rather than numbered: a call site that hides columns
 * reports positions of its own, and a server sorts by name. A grid carries a **list** of them,
 * the first deciding and the rest breaking its ties.
 */
export interface SortKey {
  column: string
  descending: boolean
}

/** The sort of a grid nobody has ordered, and a referentially stable default for one. */
export const NO_SORT: SortKey[] = []

export interface CellRef {
  row: number
  column: number
}

export interface CellRange {
  anchor: CellRef
  focus: CellRef
}

/**
 * Every rectangle the selection covers. `Ctrl`+click adds one; a drag and `Shift`+click extend
 * the **last**, which is therefore the live one and the one holding the focus cell. One list
 * rather than a single range because rows picked one at a time are not a rectangle.
 */
export interface Selection {
  ranges: CellRange[]
}

export interface CellBounds {
  rowStart: number
  rowEnd: number
  columnStart: number
  columnEnd: number
}

export interface GridData {
  columns: ResultColumn[]
  rows: Cell[][]
}

/** Identifies one grid cell, so pending edits can be looked up while rendering. */
export function cellKey(row: number, column: number): string {
  return `${row}:${column}`
}

/** The values that identify one stored row, in the shape `UpdateCell` expects. */
export function primaryKeyOf(columns: TableColumn[], row: Cell[]): Record<string, string> {
  const key: Record<string, string> = {}
  columns.forEach((column, index) => {
    if (column.primaryKey) key[column.name] = row[index] ?? ''
  })
  return key
}

export function cellText(value: Cell): string {
  return value === null ? NULL_TEXT : value
}

/**
 * How much of a value is drawn where it is only being *shown*: a cell a couple of hundred pixels
 * wide, or a menu label. Past this the reader sees nothing while the browser still shapes every
 * character to find where the ellipsis goes, so two columns of JSON documents freeze the window on
 * layout alone. A fit is capped far below this, so measuring a preview measures the same width.
 */
export const PREVIEW_LIMIT = 200

/**
 * As much of a cell as is worth drawing. What is *acted on* is never clipped — a copy, an export
 * and a filter's own condition all carry the whole value, and **View Value** reads it whole.
 */
export function cellPreview(value: Cell, limit = PREVIEW_LIMIT): string {
  const text = cellText(value)
  return text.length <= limit ? text : `${text.slice(0, limit)}…`
}

/** The inverse of `cellText`, so what a copy wrote means the same when it is pasted back. */
export function parseCell(text: string): Cell {
  return text === NULL_TEXT ? null : text
}

/**
 * The row numbers of `rows` in sorted order, for a surface whose rows are all in memory — a
 * server-paged one sorts in its `ORDER BY` instead. The keys hold in order, the first deciding
 * and the rest breaking its ties, and one naming a column the result has not got is dropped.
 * Rows from `pinned` on are the drafts a grid appended: they are not on the server, so they stay
 * where they were added, which is what keeps a paste after an Add landing on the row that was
 * added.
 *
 * NULL sorts last **whichever way a key runs**, rather than following any one engine's rule,
 * because the two engines disagree and a query result belongs to neither.
 */
export function sortedOrder(
  rows: Cell[][],
  columns: ResultColumn[],
  sort: readonly SortKey[],
  pinned = rows.length,
): number[] {
  const order = rows.map((_, index) => index)

  const keys = sort.flatMap((key) => {
    const index = columns.findIndex((column) => column.name === key.column)
    if (index < 0) return []

    return [{ index, numeric: isNumeric(columns[index]?.type ?? ''), sign: key.descending ? -1 : 1 }]
  })
  if (keys.length === 0) return order

  return [
    ...order.slice(0, pinned).sort((a, b) => {
      for (const key of keys) {
        const left = rows[a]?.[key.index] ?? null
        const right = rows[b]?.[key.index] ?? null

        if (left === null || right === null) {
          if (left === right) continue
          return left === null ? 1 : -1
        }

        const outcome = key.numeric ? Number(left) - Number(right) : left.localeCompare(right)
        if (outcome !== 0) return key.sign * outcome
      }
      return 0
    }),
    ...order.slice(pinned),
  ]
}

export function columnRange(column: number, rowCount: number): CellRange {
  return {
    anchor: { row: 0, column },
    focus: { row: Math.max(rowCount - 1, 0), column },
  }
}

export function rowRange(row: number, columnCount: number): CellRange {
  return {
    anchor: { row, column: 0 },
    focus: { row, column: Math.max(columnCount - 1, 0) },
  }
}

export function boundsOf(range: CellRange): CellBounds {
  return {
    rowStart: Math.min(range.anchor.row, range.focus.row),
    rowEnd: Math.max(range.anchor.row, range.focus.row),
    columnStart: Math.min(range.anchor.column, range.focus.column),
    columnEnd: Math.max(range.anchor.column, range.focus.column),
  }
}

export function oneRange(range: CellRange): Selection {
  return { ranges: [range] }
}

/** The cell the pointer last landed on: the live rectangle's focus. */
export function focusOf(selection: Selection | null): CellRef | null {
  return selection?.ranges.at(-1)?.focus ?? null
}

/**
 * The live rectangle's top-left, which is where a paste starts — not the focus, because a row
 * selected from the gutter focuses its *last* column and a paste into it must begin at the first.
 */
export function originOf(selection: Selection | null): CellRef | null {
  const live = selection?.ranges.at(-1)
  if (!live) return null

  const { rowStart, columnStart } = boundsOf(live)
  return { row: rowStart, column: columnStart }
}

/**
 * Every cell the selection covers, once each and in reading order, which is what a write over a
 * range acts on — `Set to NULL` over three rows is three writes and not one.
 */
export function selectedCells(
  selection: Selection | null,
  rowCount: number,
  columnCount: number,
): CellRef[] {
  const seen = new Set<string>()
  const cells: CellRef[] = []

  for (const range of selection?.ranges ?? []) {
    const bounds = boundsOf(range)

    for (let row = Math.max(bounds.rowStart, 0); row <= Math.min(bounds.rowEnd, rowCount - 1); row++) {
      for (
        let column = Math.max(bounds.columnStart, 0);
        column <= Math.min(bounds.columnEnd, columnCount - 1);
        column++
      ) {
        const key = cellKey(row, column)
        if (seen.has(key)) continue

        seen.add(key)
        cells.push({ row, column })
      }
    }
  }

  return cells
}

/** Every row the selection touches, once each and in order, which is what a delete acts on. */
export function selectedRows(selection: Selection | null): number[] {
  const rows = new Set<number>()

  for (const range of selection?.ranges ?? []) {
    const { rowStart, rowEnd } = boundsOf(range)
    for (let row = rowStart; row <= rowEnd; row++) rows.add(row)
  }

  return [...rows].sort((a, b) => a - b)
}

/** One rectangle of the selection: its cells, and the names of the columns they sit under. */
export interface CellBlock {
  columns: string[]
  rows: Cell[][]
}

/**
 * The selection as rectangles of values, top to bottom, so rows picked in any order still read
 * in the order they are drawn. Every format a copy writes is built from these — the names are
 * carried because a CSV and a JSON object need them and a plain copy does not.
 */
export function selectedBlocks(selection: Selection, data: GridData): CellBlock[] {
  return selection.ranges
    .map(boundsOf)
    .sort((a, b) => a.rowStart - b.rowStart || a.columnStart - b.columnStart)
    .map((bounds) => block(bounds, data))
}

export function toTSV(selection: Selection, data: GridData): string {
  return selectedBlocks(selection, data)
    .map((one) => one.rows.map((row) => row.map(cellText).join('\t')).join('\n'))
    .join('\n')
}

function block(bounds: CellBounds, data: GridData): CellBlock {
  const columns: string[] = []
  for (let column = bounds.columnStart; column <= bounds.columnEnd; column++) {
    const name = data.columns[column]?.name
    if (name !== undefined) columns.push(name)
  }

  const rows: Cell[][] = []
  for (let row = bounds.rowStart; row <= bounds.rowEnd; row++) {
    const source = data.rows[row]
    if (!source) continue

    const cells: Cell[] = []
    for (let column = bounds.columnStart; column <= bounds.columnEnd; column++) {
      cells.push(source[column] ?? null)
    }
    rows.push(cells)
  }

  return { columns, rows }
}
