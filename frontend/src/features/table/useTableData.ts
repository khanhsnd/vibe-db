import { useCallback, useEffect, useState } from 'react'

import type { Namespace } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { ALL_ROWS, DEFAULT_PAGE_SIZE } from '@/bridge/rows'
import { tables, type TablePage } from '@/bridge/tables'

export function useTableData(sessionId: string, namespace: Namespace, table: string) {
  const { database, schema } = namespace

  const [page, setPage] = useState(0)
  const [limit, setLimit] = useState<number>(DEFAULT_PAGE_SIZE)
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
  }, [sessionId, database, schema, table, limit, page, nonce])

  const refresh = useCallback(() => setNonce((current) => current + 1), [])

  const goToPage = useCallback((next: number) => setPage(Math.max(0, next)), [])

  const changeLimit = useCallback((next: number) => {
    setLimit(next)
    setPage(0)
  }, [])

  return { data, loading, error, page, limit, goToPage, changeLimit, refresh }
}
