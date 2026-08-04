import { useCallback, useMemo, useState } from 'react'

import { useSettingsStore } from '@/stores/settings'

import { pageStep } from './paging'
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
  // Read once, because the preference is what a page *starts* at: changing it in Settings must not
  // move a result the reader has already paged through.
  const [size, setSize] = useState<number>(() => useSettingsStore.getState().settings.pageSize)

  const spanOf = (count: number) => pageStep(size, count)

  const step = spanOf(rows.length)
  const pageCount = Math.max(1, Math.ceil(rows.length / step))
  const current = Math.min(page, pageCount - 1)
  const offset = current * step

  const window = useMemo(() => rows.slice(offset, offset + step), [rows, offset, step])

  const goToPage = useCallback((next: number) => setPage(Math.max(0, next)), [])

  /**
   * Where the last of `total` rows will sit, so an appended row can be paged to and selected in
   * one go. It takes the count rather than reading `rows`, because the caller is adding to them.
   */
  const lastPlace = (total: number) => {
    const span = spanOf(total)
    const page = Math.max(0, Math.ceil(total / span) - 1)
    return { page, row: total - 1 - page * span }
  }

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
    lastPlace,
    changeSize,
  }
}
