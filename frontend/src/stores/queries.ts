import { create } from 'zustand'

import { toAppError, type AppError } from '@/bridge/errors'
import { queries, type QueryRequest, type QueryResult } from '@/bridge/query'

export interface QueryRun {
  running: boolean
  result: QueryResult | null
  error: AppError | null
  /** What produced this run, so Refresh replays it instead of whatever the editor now holds. */
  request: RunRequest | null
}

/** A stable snapshot for tabs that have not run anything, so selectors stay referentially equal. */
export const IDLE_RUN: QueryRun = { running: false, result: null, error: null, request: null }

type RunRequest = Omit<QueryRequest, 'runId'>

interface QueriesState {
  runs: Record<string, QueryRun>
  /** Run id of the statement in flight per tab, so Stop knows what to cancel. */
  pending: Record<string, string>

  execute: (tabId: string, sessionId: string, request: RunRequest) => Promise<void>
  /**
   * Replays the statement that produced the rows on screen. `maxRows` overrides what it asked
   * for, which is how the pager reaches rows the first read stopped short of.
   */
  rerun: (tabId: string, sessionId: string, maxRows?: number) => Promise<void>
  cancel: (tabId: string) => void
  discard: (tabId: string) => void
}

export const useQueriesStore = create<QueriesState>((set, get) => ({
  runs: {},
  pending: {},

  execute: async (tabId, sessionId, request) => {
    if (get().runs[tabId]?.running) return

    const runId = crypto.randomUUID()
    set((s) => ({
      runs: {
        ...s.runs,
        // The previous rows stay on screen until the new ones land.
        [tabId]: { running: true, result: s.runs[tabId]?.result ?? null, error: null, request },
      },
      pending: { ...s.pending, [tabId]: runId },
    }))

    try {
      const result = await queries.run(sessionId, { ...request, runId })
      set((s) => ({
        runs: { ...s.runs, [tabId]: { running: false, result, error: null, request } },
      }))
    } catch (thrown) {
      set((s) => ({
        runs: {
          ...s.runs,
          [tabId]: { running: false, result: null, error: toAppError(thrown), request },
        },
      }))
    } finally {
      set((s) => ({ pending: omit(s.pending, tabId) }))
    }
  },

  rerun: async (tabId, sessionId, maxRows) => {
    const request = get().runs[tabId]?.request
    if (!request) return

    await get().execute(tabId, sessionId, maxRows === undefined ? request : { ...request, maxRows })
  },

  cancel: (tabId) => {
    const runId = get().pending[tabId]
    if (runId) void queries.cancel(runId)
  },

  discard: (tabId) =>
    set((s) => ({ runs: omit(s.runs, tabId), pending: omit(s.pending, tabId) })),
}))

function omit<T>(source: Record<string, T>, key: string): Record<string, T> {
  const next = { ...source }
  delete next[key]
  return next
}
