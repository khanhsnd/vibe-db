import { useCallback, useEffect, useRef, useState } from 'react'

import { toAppError, type AppError } from '@/bridge/errors'

/**
 * One read of a server that is worth asking again: the process list, the slow list, the status, the
 * variables. All of them are read **fresh per tab** and never cached in the store, for the reason the
 * table list is — what the reader opened the pane for is the number as it is *now*, and a stale one is
 * worse than a second query.
 *
 * `every` is how many seconds apart to ask again, and `0` never asks twice. A re-read that fails
 * leaves the rows already drawn where they are and puts the reason beside them: a monitor that blanked
 * itself the moment a server hiccuped would be a monitor nobody could watch.
 *
 * The clock is **stopped by the caller** rather than by anything here — see `MonitorView`, which holds
 * it while a row is picked, because a list that moved under a pick would kill something the reader did
 * not point at.
 *
 * `asking` names **what is being read** — the pane, and the database where that matters. It is the one
 * thing here that is depended on rather than held in a ref, and everything below follows from it: the
 * read is a closure over the session and the pane, so it is a new function on every render and
 * depending on *it* would ask the server again per render. Naming the question instead is what lets a
 * reader switch panes or databases and simply see the answer.
 */
export function useServerReport<T>(read: () => Promise<T>, every: number, asking: string) {
  const [data, setData] = useState<T | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<AppError | null>(null)
  const [nonce, setNonce] = useState(0)

  const latest = useRef(read)
  latest.current = read

  /**
   * Which question the answer coming back belongs to. A reader who moves through three databases
   * faster than a busy server answers has three reads in flight, and without this the slowest of them
   * would land last and win — leaving the pane showing a database nobody is looking at any more.
   */
  const asked = useRef(0)

  const load = useCallback(async () => {
    asked.current += 1
    const mine = asked.current

    setLoading(true)
    try {
      const next = await latest.current()
      if (mine !== asked.current) return
      setData(next)
      setError(null)
    } catch (thrown) {
      if (mine !== asked.current) return
      setError(toAppError(thrown))
    } finally {
      if (mine === asked.current) setLoading(false)
    }
  }, [])

  /** The question the rows on screen answered, so that a new one can be told from a re-read. */
  const showing = useRef(asking)

  // Reading and the clock are **two effects**, which is the whole of what keeps a held clock held:
  // one read happens when the pane opens, when the question changes and whenever Refresh asks, and
  // the interval only ever starts or stops. Folded into one effect, turning the clock off — which is
  // what picking a row does — would itself have been a read, moving the very rows the reader had just
  // picked.
  useEffect(() => {
    // Rows that answered a **different question** go before the new one is asked: their columns are
    // another pane's, and a grid left standing under a fresh header would show one answer under
    // another's heading until the read came back. A *re-read of the same question* is the opposite
    // case and keeps its rows — which is what makes a hiccuping server watchable.
    if (showing.current !== asking) {
      showing.current = asking
      setData(null)
      setError(null)
    }
    void load()
  }, [load, asking, nonce])

  useEffect(() => {
    if (every <= 0) return
    const timer = setInterval(() => void load(), every * 1000)
    return () => clearInterval(timer)
  }, [load, every])

  const refresh = useCallback(() => setNonce((current) => current + 1), [])

  return { data, loading, error, refresh }
}
