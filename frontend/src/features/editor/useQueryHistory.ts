import { useCallback, useEffect, useState } from 'react'

import { toAppError, type AppError } from '@/bridge/errors'
import { queryHistory, type HistoryEntry } from '@/bridge/history'

/**
 * The statements this connection has run, newest first.
 *
 * It is read on mount rather than held in a store: the pane is drawn only while the reader is
 * looking at it, so every open is a fresh log and an Execute never has to tell it anything.
 *
 * The log itself is every connection's — a statement is kept with the namespace it ran in — and
 * this narrows it to one, because a statement of another server names tables that are not here.
 */
export function useQueryHistory(profileId: string) {
  const [nonce, setNonce] = useState(0)
  const [entries, setEntries] = useState<HistoryEntry[] | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<AppError | null>(null)

  useEffect(() => {
    let active = true
    setLoading(true)

    queryHistory
      .list()
      .then((log) => {
        if (!active) return
        setEntries(log.filter((entry) => entry.profileId === profileId))
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
  }, [profileId, nonce])

  const refresh = useCallback(() => setNonce((current) => current + 1), [])

  const clear = useCallback(async () => {
    try {
      await queryHistory.clear()
      refresh()
    } catch (thrown) {
      setError(toAppError(thrown))
    }
  }, [refresh])

  return { entries, loading, error, refresh, clear }
}
