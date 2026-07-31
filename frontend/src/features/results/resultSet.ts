import type { Cell, TableColumn } from '@/bridge/tables'

export type { Cell }

export const NULL_TEXT = '(Null)'

export interface ResultColumn {
  name: string
  /** The bare type, which classifies the column. `fullType` carries its length and modifiers. */
  type: string
  fullType: string
}

export interface CellRef {
  row: number
  column: number
}

export interface CellRange {
  anchor: CellRef
  focus: CellRef
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

export function boundsOf(range: CellRange): CellBounds {
  return {
    rowStart: Math.min(range.anchor.row, range.focus.row),
    rowEnd: Math.max(range.anchor.row, range.focus.row),
    columnStart: Math.min(range.anchor.column, range.focus.column),
    columnEnd: Math.max(range.anchor.column, range.focus.column),
  }
}

export function toTSV(range: CellRange, data: GridData): string {
  const { rowStart, rowEnd, columnStart, columnEnd } = boundsOf(range)
  const lines: string[] = []

  for (let row = rowStart; row <= rowEnd; row++) {
    const source = data.rows[row]
    if (!source) continue

    const cells: string[] = []
    for (let column = columnStart; column <= columnEnd; column++) {
      cells.push(cellText(source[column] ?? null))
    }
    lines.push(cells.join('\t'))
  }

  return lines.join('\n')
}
