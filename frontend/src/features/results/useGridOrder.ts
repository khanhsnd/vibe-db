import { useMemo } from 'react'

import { sortedOrder, type Cell, type ResultColumn, type SortKey } from './resultSet'

/**
 * The rows of a grid that has no statement of its own to order — a query result, the history log —
 * as a list of row numbers rather than reordered rows: every pending change is keyed by the number
 * a row had when it was read.
 *
 * `pinned` is how many rows are sorted at all. Anything past it stays where it is, which is what
 * keeps a draft row — one the reader is still filling in — at the end of the grid.
 */
export function useGridOrder(
  rows: Cell[][],
  columns: ResultColumn[],
  sort: readonly SortKey[],
  pinned = rows.length,
): number[] {
  return useMemo(() => sortedOrder(rows, columns, sort, pinned), [rows, columns, sort, pinned])
}
