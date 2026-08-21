import { useCallback, useEffect, useState } from 'react'

import { IDLE_METRICS, useMetricsStore } from '@/stores/metrics'

/**
 * The clock behind the four charts. What it collects is `useMetricsStore`'s, and the split is the
 * whole point: only the tab in front is mounted, so a history held here would go the moment the
 * reader looked at another tab — and a chart, unlike the four lists beside it, *is* the minutes that
 * came before.
 *
 * It is **not** `useServerReport`, which holds one read at a time and throws the last one away
 * because a process list is a picture of now.
 *
 * `watching` is whether the reader is on the charts pane at all. Nothing is asked while it is false,
 * so opening a monitor on the process list costs no metrics read, and a tab left in the background
 * asks a production server nothing. What it had already collected stays.
 *
 * See [docs/features/metrics.md](../../../../docs/features/metrics.md).
 */
export function useMetricHistory(
  tabId: string,
  sessionId: string,
  every: number,
  watching: boolean,
) {
  const run = useMetricsStore((s) => s.runs[tabId] ?? IDLE_METRICS)
  const ask = useMetricsStore((s) => s.read)
  const [nonce, setNonce] = useState(0)

  const read = useCallback(() => void ask(tabId, sessionId), [ask, tabId, sessionId])

  // Reading and the clock are two effects, which is `useServerReport`'s arrangement and is here for
  // its reason: arriving on the pane and pressing Refresh are reads, and changing the interval is
  // only ever starting or stopping a timer.
  useEffect(() => {
    if (!watching) return
    read()
  }, [read, watching, nonce])

  useEffect(() => {
    if (!watching || every <= 0) return
    const timer = setInterval(read, every * 1000)
    return () => clearInterval(timer)
  }, [read, watching, every])

  const refresh = useCallback(() => setNonce((current) => current + 1), [])

  return { history: run.history, error: run.error, reading: run.reading, refresh }
}
