import { useCallback, useEffect, useState } from 'react'

import { toAppError, type AppError } from '@/bridge/errors'
import { tables, type Cell, type TableColumn, type TablePage } from '@/bridge/tables'
import type { CellRef } from '@/features/results/resultSet'

export const PAGE_SIZES = [100, 500, 1000] as const
export const DEFAULT_PAGE_SIZE = 100

export function useTableData(sessionId: string, database: string, table: string) {
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
      .loadRows(sessionId, { database, table, limit, offset: page * limit })
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
  }, [sessionId, database, table, limit, page, nonce])

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
    [data, sessionId, database, table, refresh],
  )

  return { data, loading, saving, error, page, limit, goToPage, changeLimit, refresh, updateCell }
}

function primaryKeyOf(columns: TableColumn[], row: Cell[]): Record<string, string> {
  const key: Record<string, string> = {}
  columns.forEach((column, index) => {
    if (column.primaryKey) key[column.name] = row[index] ?? ''
  })
  return key
}
