import { useCallback, useEffect, useState } from 'react'

import type { Namespace } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { ALL_ROWS, DEFAULT_PAGE_SIZE } from '@/bridge/rows'
import { tables, type Cell, type TablePage } from '@/bridge/tables'
import { primaryKeyOf, type CellRef } from '@/features/results/resultSet'

export function useTableData(sessionId: string, namespace: Namespace, table: string) {
  const { database, schema } = namespace

  const [page, setPage] = useState(0)
  const [limit, setLimit] = useState<number>(DEFAULT_PAGE_SIZE)
  const [nonce, setNonce] = useState(0)
  const [data, setData] = useState<TablePage | null>(null)
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
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

  const updateCell = useCallback(
    async (ref: CellRef, value: Cell) => {
      const column = data?.columns[ref.column]
      const row = data?.rows[ref.row]
      if (!column || !row) return

      setSaving(true)
      try {
        await tables.updateCell(sessionId, {
          database,
          schema,
          table,
          column: column.name,
          value,
          key: primaryKeyOf(data.columns, row),
        })
        setError(null)
        refresh()
      } catch (thrown) {
        setError(toAppError(thrown))
      } finally {
        setSaving(false)
      }
    },
    [data, sessionId, database, schema, table, refresh],
  )

  return { data, loading, saving, error, page, limit, goToPage, changeLimit, refresh, updateCell }
}
