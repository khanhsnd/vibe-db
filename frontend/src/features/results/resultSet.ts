import type { Cell, TableColumn } from '@/bridge/tables'

import { isNumeric } from './columnType'

export type { Cell }

export const NULL_TEXT = '(Null)'

export interface ResultColumn {
  name: string
  /** The bare type, which classifies the column. `fullType` carries its length and modifiers. */
  type: string
  fullType: string
}

/**
 * The one column a grid is sorted by, named rather than numbered: a call site that hides
 * columns reports positions of its own, and a server sorts by name.
 */
export interface SortKey {
  column: string
  descending: boolean
}

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

/** The inverse of `cellText`, so what a copy wrote means the same when it is pasted back. */
export function parseCell(text: string): Cell {
  return text === NULL_TEXT ? null : text
}

/**
 * The row numbers of `rows` in sorted order, for a surface whose rows are all in memory — a
 * server-paged one sorts in its `ORDER BY` instead. Rows from `pinned` on are the drafts a
 * grid appended: they are not on the server, so they stay where they were added, which is
 * what keeps a paste after an Add landing on the row that was added.
 *
 * NULL sorts last either way, rather than following any one engine's rule, because the two
 * engines disagree and a query result belongs to neither.
 */
export function sortedOrder(
  rows: Cell[][],
  columns: ResultColumn[],
  sort: SortKey | null,
  pinned = rows.length,
): number[] {
  const order = rows.map((_, index) => index)
  const index = columns.findIndex((column) => column.name === sort?.column)
  if (!sort || index < 0) return order

  const numeric = isNumeric(columns[index]?.type ?? '')
  const sign = sort.descending ? -1 : 1

  return [
    ...order.slice(0, pinned).sort((a, b) => {
      const left = rows[a]?.[index] ?? null
      const right = rows[b]?.[index] ?? null

      if (left === null || right === null) return left === right ? 0 : left === null ? 1 : -1
      return sign * (numeric ? Number(left) - Number(right) : left.localeCompare(right))
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

/** Every row the selection touches, once each and in order, which is what a delete acts on. */
export function selectedRows(selection: Selection | null): number[] {
  const rows = new Set<number>()

  for (const range of selection?.ranges ?? []) {
    const { rowStart, rowEnd } = boundsOf(range)
    for (let row = rowStart; row <= rowEnd; row++) rows.add(row)
  }

  return [...rows].sort((a, b) => a - b)
}

/** Rectangles top to bottom, so rows picked in any order still copy in the order they read. */
export function toTSV(selection: Selection, data: GridData): string {
  return selection.ranges
    .map(boundsOf)
    .sort((a, b) => a.rowStart - b.rowStart || a.columnStart - b.columnStart)
    .map((bounds) => block(bounds, data))
    .join('\n')
}

function block(bounds: CellBounds, data: GridData): string {
  const lines: string[] = []

  for (let row = bounds.rowStart; row <= bounds.rowEnd; row++) {
    const source = data.rows[row]
    if (!source) continue

    const cells: string[] = []
    for (let column = bounds.columnStart; column <= bounds.columnEnd; column++) {
      cells.push(cellText(source[column] ?? null))
    }
    lines.push(cells.join('\t'))
  }

  return lines.join('\n')
}
