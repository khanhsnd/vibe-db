import { useCallback, useEffect, useState } from 'react'

import { compare, compareSide, type DataDiff, type StructureDiff } from '@/bridge/compare'
import { toAppError, type AppError } from '@/bridge/errors'
import { useConnectionsStore } from '@/stores/connections'
import type { CompareMode, TableTarget } from '@/stores/tabs'

export interface Comparison {
  /** Exactly one of the two is filled — whichever half the tab is showing. */
  structure: StructureDiff | null
  data: DataDiff | null
  loading: boolean
  error: AppError | null
  /** Whether both sides are open at all, which is the one thing a read cannot go without. */
  connected: boolean
  refresh: () => void
  /** The two tables this comparison is of, carried so a migration need not be told them twice. */
  left: TableTarget
  right: TableTarget
  /**
   * How many times this comparison has been re-read. Anything else that reads about the same pair
   * — the DDL diff — watches it, so an Apply that refreshed the comparison refreshes them too
   * rather than leaving a declaration on screen that is no longer the table's.
   */
  token: number
}

/**
 * Reads one half of a comparison, and re-reads it when the half or the row count changes.
 *
 * **Only the half being shown is read.** Two catalogue reads are cheap and two bounded table reads
 * are not, so switching to Data is a read the reader asked for rather than one the tab made on the
 * way in — and switching back does not pay for the rows a second time, having dropped them.
 *
 * The two sides carry their own sessions, because they need not be on one server.
 */
export function useCompare(
  left: TableTarget,
  right: TableTarget,
  mode: CompareMode,
  rows: number,
): Comparison {
  const leftSession = useConnectionsStore((s) => s.sessions[left.profileId]?.id ?? '')
  const rightSession = useConnectionsStore((s) => s.sessions[right.profileId]?.id ?? '')

  const [structure, setStructure] = useState<StructureDiff | null>(null)
  const [data, setData] = useState<DataDiff | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState<AppError | null>(null)
  const [token, setToken] = useState(0)

  const connected = leftSession !== '' && rightSession !== ''
  const refresh = useCallback(() => setToken((one) => one + 1), [])

  // The row count belongs to the data half alone, so a count changed while the *shapes* are on
  // screen is not a reason to read them again: it is zero until something is reading rows by it.
  const limit = mode === 'data' ? rows : 0

  useEffect(() => {
    if (!connected) return

    let live = true
    const from = compareSide(leftSession, left.namespace, left.table)
    const to = compareSide(rightSession, right.namespace, right.table)

    setLoading(true)
    setError(null)
    setStructure(null)
    setData(null)

    const read =
      mode === 'structure'
        ? compare.structure(from, to).then((diff) => {
            if (live) setStructure(diff)
          })
        : compare.data(from, to, limit).then((diff) => {
            if (live) setData(diff)
          })

    void read
      .catch((thrown) => {
        if (live) setError(toAppError(thrown))
      })
      .finally(() => {
        if (live) setLoading(false)
      })

    // A read whose tab has moved on — to the other half, to another row count, to nothing at all
    // — must not land on the surface that replaced it.
    return () => {
      live = false
    }
  }, [
    connected,
    leftSession,
    rightSession,
    left.namespace,
    left.table,
    right.namespace,
    right.table,
    mode,
    limit,
    token,
  ])

  return { structure, data, loading, error, connected, refresh, left, right, token }
}
