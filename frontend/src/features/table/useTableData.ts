import { useCallback, useEffect, useState } from 'react'

import type { Namespace } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { ALL_ROWS, DEFAULT_PAGE_SIZE } from '@/bridge/rows'
import { tables, type TablePage } from '@/bridge/tables'
import type { SortKey } from '@/features/results/resultSet'

const NO_SORT = { column: '', descending: false }

/**
 * The rows are paged on the server, so the sort has to be too: ordering the page in hand
 * would only shuffle the hundred rows the reader can already see.
 */
export function useTableData(sessionId: string, namespace: Namespace, table: string) {
  const { database, schema } = namespace

  const [page, setPage] = useState(0)
  const [limit, setLimit] = useState<number>(DEFAULT_PAGE_SIZE)
  const [sort, setSort] = useState<SortKey | null>(null)
  const [nonce, setNonce] = useState(0)
  const [data, setData] = useState<TablePage | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<AppError | null>(null)

  useEffect(() => {
    let active = true
    setLoading(true)

    tables
      .loadRows(sessionId, {
        database,
        schema,
        table,
        limit,
        offset: limit === ALL_ROWS ? 0 : page * limit,
        sort: sort ?? NO_SORT,
      })
      .then((next) => {
        if (!active) return
        setData(next)
        setError(null)
      })
      .catch((thrown: unknown) => {
        if (active) setError(toAppError(thrown))
      })
      .finally(() => {
        if (active) setLoading(false)
      })

    return () => {
      active = false
    }
  }, [sessionId, database, schema, table, limit, page, sort, nonce])

  const refresh = useCallback(() => setNonce((current) => current + 1), [])

  const goToPage = useCallback((next: number) => setPage(Math.max(0, next)), [])

  const changeLimit = useCallback((next: number) => {
    setLimit(next)
    setPage(0)
  }, [])

  /** A new order makes the page number meaningless, so the reader goes back to the first. */
  const changeSort = useCallback((next: SortKey | null) => {
    setSort(next)
    setPage(0)
  }, [])

  return { data, loading, error, page, limit, sort, goToPage, changeLimit, changeSort, refresh }
}
