import { useMemo } from 'react'

import { activeFilter, matchesFilter, type Group } from './filter'
import { sortedOrder, type Cell, type ResultColumn, type SortKey } from './resultSet'

/**
 * The rows of a grid that has no statement of its own to order or narrow — a query result, the
 * history log — as a list of row numbers rather than reordered rows: every pending change is keyed
 * by the number a row had when it was read.
 *
 * `pinned` is how many rows are sorted and filtered at all. Anything past it stays where it is and
 * is never filtered away, which is what keeps a draft row — one the reader is still filling in —
 * from vanishing mid-typing.
 */
export function useGridOrder(
  rows: Cell[][],
  columns: ResultColumn[],
  sort: readonly SortKey[],
  criteria: Group,
  pinned = rows.length,
): number[] {
  return useMemo(() => {
    // Pruned once per read rather than once per row: what is left is exactly the tree a table
    // read would have put in its `WHERE`.
    const filter = activeFilter(criteria)

    return sortedOrder(rows, columns, sort, pinned).filter(
      (row) => row >= pinned || matchesFilter(rows[row] ?? [], columns, filter),
    )
  }, [rows, columns, sort, criteria, pinned])
}
