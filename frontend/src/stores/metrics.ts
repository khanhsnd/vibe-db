import { create } from 'zustand'

import { toAppError, type AppError } from '@/bridge/errors'
import { loadServerMetrics } from '@/bridge/monitor'
import { remember, type Reading } from '@/features/monitor/metrics'

/** What one monitor tab has collected so far, and how its last read went. */
export interface MetricRun {
  history: readonly Reading[]
  reading: boolean
  error: AppError | null
}

/** A stable snapshot for a tab that has read nothing yet, so selectors stay equal. */
export const IDLE_METRICS: MetricRun = { history: [], reading: false, error: null }

interface MetricsState {
  runs: Record<string, MetricRun>

  read: (tabId: string, sessionId: string) => Promise<void>
  discard: (tabId: string) => void
}

/**
 * The readings behind each monitor tab's four charts.
 *
 * It is a **store** and not the view's own state for the reason a Find in Schema's hits are one:
 * only the tab in front is mounted, so readings held in a component go the moment the reader looks at
 * a query tab. That costs nothing on the four list panes beside the charts — a process list is a
 * picture of now, and reading it again is one read — but a chart *is* the minutes that came before,
 * and there is nothing to read again. Keyed by tab id and dropped when the tab closes, which is what
 * makes "the readings live in the tab and go when it is closed" true.
 *
 * The clock is still the view's. Nothing is asked while nobody is looking at a chart, so a tab left
 * in the background stops collecting and keeps what it had; the gap that leaves is drawn as the gap
 * it is, the axis being time rather than a count of readings — see `spanAt`.
 *
 * See [docs/features/metrics.md](../../../docs/features/metrics.md).
 */
export const useMetricsStore = create<MetricsState>((set, get) => ({
  runs: {},

  read: async (tabId, sessionId) => {
    // A server slower than the clock is asked **once and waited for** rather than asked again on
    // top: a pile of overlapping reads would answer in whatever order they finished, and a reading
    // stamped out of order is one `remember` drops.
    if (get().runs[tabId]?.reading) return

    set((s) => ({
      runs: { ...s.runs, [tabId]: { ...(s.runs[tabId] ?? IDLE_METRICS), reading: true } },
    }))

    try {
      const next = await loadServerMetrics(sessionId)
      set((s) => {
        const held = s.runs[tabId]
        // The tab was closed while the server was answering. Writing the answer back would put a
        // history behind a tab that is not there any more.
        if (held === undefined) return {}
        return {
          runs: {
            ...s.runs,
            [tabId]: { history: remember(held.history, next), reading: false, error: null },
          },
        }
      })
    } catch (thrown) {
      set((s) => {
        const held = s.runs[tabId]
        if (held === undefined) return {}
        // The history stays exactly where it was. A chart that emptied itself the moment a server
        // hiccoughed would throw away the very minutes the reader opened it to look at.
        return {
          runs: { ...s.runs, [tabId]: { ...held, reading: false, error: toAppError(thrown) } },
        }
      })
    }
  },

  discard: (tabId) => set((s) => ({ runs: omit(s.runs, tabId) })),
}))

function omit<T>(source: Record<string, T>, key: string): Record<string, T> {
  const next = { ...source }
  delete next[key]
  return next
}
