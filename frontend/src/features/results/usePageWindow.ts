import { useCallback, useMemo, useState } from 'react'

import { ALL_ROWS, DEFAULT_PAGE_SIZE } from '@/bridge/rows'

import type { Cell } from './resultSet'

/**
 * Cuts one page out of rows already in memory, for a surface whose rows arrive whole. A query
 * result is fetched once, so paging it must not read again — unlike the table view, which walks
 * the server with `LIMIT … OFFSET …`.
 *
 * The page is clamped rather than reset, because the rows it counts change identity on every
 * pending edit: resetting on that would throw the reader back to page one for a keystroke.
 */
export function usePageWindow(rows: Cell[][]) {
  const [page, setPage] = useState(0)
  const [size, setSize] = useState<number>(DEFAULT_PAGE_SIZE)

  const step = size === ALL_ROWS ? Math.max(rows.length, 1) : size
  const pageCount = Math.max(1, Math.ceil(rows.length / step))
  const current = Math.min(page, pageCount - 1)
  const offset = current * step

  const window = useMemo(() => rows.slice(offset, offset + step), [rows, offset, step])

  const goToPage = useCallback((next: number) => setPage(Math.max(0, next)), [])

  const changeSize = useCallback((next: number) => {
    setSize(next)
    setPage(0)
  }, [])

  return {
    /** The page in force, which is never past the last one. */
    page: current,
    pageCount,
    size,
    offset,
    total: rows.length,
    rows: window,
    goToPage,
    changeSize,
  }
}
