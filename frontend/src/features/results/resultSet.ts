export interface ResultColumn {
  name: string
  type: string
}

export type Cell = string | null

export interface ResultSet {
  columns: ResultColumn[]
  rows: Cell[][]
  elapsedMs: number
  truncated: boolean
}

const NUMERIC_TYPE =
  /^(TINYINT|SMALLINT|MEDIUMINT|INT|INTEGER|BIGINT|DECIMAL|NUMERIC|FLOAT|DOUBLE|BIT|YEAR)/i

export function isNumeric(type: string): boolean {
  return NUMERIC_TYPE.test(type)
}
