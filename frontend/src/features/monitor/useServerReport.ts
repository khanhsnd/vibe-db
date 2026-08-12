import { useCallback, useEffect, useRef, useState } from 'react'

import { toAppError, type AppError } from '@/bridge/errors'

/**
 * One read of a server that is worth asking again: the process list, the status, the variables. All
 * three are read **fresh per tab** and never cached in the store, for the reason the table list is —
 * what the reader opened the pane for is the number as it is *now*, and a stale one is worse than a
 * second query.
 *
 * `every` is how many seconds apart to ask again, and `0` never asks twice. A re-read that fails
 * leaves the rows already drawn where they are and puts the reason beside them: a monitor that blanked
 * itself the moment a server hiccuped would be a monitor nobody could watch.
 *
 * The clock is **stopped by the caller** rather than by anything here — see `MonitorView`, which holds
 * it while a row is picked, because a list that moved under a pick would kill something the reader did
 * not point at.
 */
export function useServerReport<T>(read: () => Promise<T>, every: number) {
  const [data, setData] = useState<T | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<AppError | null>(null)
  const [nonce, setNonce] = useState(0)

  // The read is a closure over a session and a pane, so it changes on every render of the view. It is
  // held in a ref rather than depended on: an effect that re-ran per render would ask the server
  // again per render.
  const latest = useRef(read)
  latest.current = read

  const load = useCallback(async () => {
    setLoading(true)
    try {
      const next = await latest.current()
      setData(next)
      setError(null)
    } catch (thrown) {
      setError(toAppError(thrown))
    } finally {
      setLoading(false)
    }
  }, [])

  // Reading and the clock are **two effects**, which is the whole of what keeps a held clock held:
  // one read happens when the pane opens and whenever Refresh asks, and the interval only ever starts
  // or stops. Folded into one effect, turning the clock off — which is what picking a row does — would
  // itself have been a read, moving the very rows the reader had just picked.
  useEffect(() => {
    void load()
  }, [load, nonce])

  useEffect(() => {
    if (every <= 0) return
    const timer = setInterval(() => void load(), every * 1000)
    return () => clearInterval(timer)
  }, [load, every])

  const refresh = useCallback(() => setNonce((current) => current + 1), [])

  return { data, loading, error, refresh }
}
