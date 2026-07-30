import type { Cell } from '@/bridge/tables'

export type { Cell }

export interface ResultColumn {
  name: string
  type: string
}

export interface CellRef {
  row: number
  column: number
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
