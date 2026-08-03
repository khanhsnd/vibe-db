import { useCallback, useEffect, useState } from 'react'

import type { Namespace } from '@/bridge/connections'
import { toAppError, type AppError } from '@/bridge/errors'
import { tables, type TableList } from '@/bridge/tables'

/**
 * What the catalogue keeps about the tables of one namespace. It is read fresh per tab rather
 * than cached in the store like the tree's names are: a size and a row count are what the reader
 * opened the list to see, and a stale one is worse than a second query.
 */
export function useTableList(sessionId: string, namespace: Namespace) {
  const { database, schema } = namespace

  const [nonce, setNonce] = useState(0)
  const [data, setData] = useState<TableList | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<AppError | null>(null)

  useEffect(() => {
    let active = true
    setLoading(true)

    tables
      .loadList(sessionId, { database, schema })
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
  }, [sessionId, database, schema, nonce])

  const refresh = useCallback(() => setNonce((current) => current + 1), [])

  return { data, loading, error, refresh }
}
