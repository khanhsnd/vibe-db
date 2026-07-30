import type { Cell } from '@/bridge/tables'

export type { Cell }

export const NULL_TEXT = 'NULL'

export interface ResultColumn {
  name: string
  type: string
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

export interface ResultSet extends GridData {
  elapsedMs: number
  truncated: boolean
}

const NUMERIC_TYPE =
  /^(TINYINT|SMALLINT|MEDIUMINT|INT|INTEGER|BIGINT|DECIMAL|NUMERIC|FLOAT|DOUBLE|BIT|YEAR)/i

export function isNumeric(type: string): boolean {
  return NUMERIC_TYPE.test(type)
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
