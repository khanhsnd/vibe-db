import { create } from 'zustand'

import { toAppError, type AppError } from '@/bridge/errors'
import { search, type SearchRequest, type SearchResult } from '@/bridge/search'

/** What a search asked for, which is everything about it but the id it is stopped by. */
export type FindRequest = Omit<SearchRequest, 'runId'>

export interface SearchRun {
  running: boolean
  result: SearchResult | null
  error: AppError | null
  /**
   * What found the results on screen, so the pane says what it was looking for rather than what
   * the form now holds — a reader who has typed a second word has not searched for it yet.
   */
  request: FindRequest | null
}

/** A stable snapshot for a tab that has searched for nothing yet, so selectors stay equal. */
export const IDLE_SEARCH: SearchRun = { running: false, result: null, error: null, request: null }

interface SearchesState {
  runs: Record<string, SearchRun>
  /** The run id in flight per tab, so Stop knows what to cancel. */
  pending: Record<string, string>

  find: (tabId: string, sessionId: string, request: FindRequest) => Promise<void>
  stop: (tabId: string) => void
  discard: (tabId: string) => void
}

/**
 * What each Find in Schema tab has found. It is a store rather than the view's own state for the
 * reason a query tab's rows are one: only the tab in front is mounted, and a search over a whole
 * schema is far too long a read to lose by looking at another tab.
 *
 * What was *asked* lives on the tab itself (`FindTab`), exactly as a query tab keeps its SQL: the
 * question is what the tab is, and the answer is what this holds.
 */
export const useSearchesStore = create<SearchesState>((set, get) => ({
  runs: {},
  pending: {},

  find: async (tabId, sessionId, request) => {
    if (get().runs[tabId]?.running) return

    const runId = crypto.randomUUID()
    set((s) => ({
      // The hits already found stay on screen until the new ones land, which is what the grid
      // beside a re-run does everywhere else.
      runs: {
        ...s.runs,
        [tabId]: { running: true, result: s.runs[tabId]?.result ?? null, error: null, request },
      },
      pending: { ...s.pending, [tabId]: runId },
    }))

    try {
      const result = await search.find(sessionId, { ...request, runId })
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

  stop: (tabId) => {
    const runId = get().pending[tabId]
    if (runId) void search.stop(runId)
  },

  discard: (tabId) =>
    set((s) => ({ runs: omit(s.runs, tabId), pending: omit(s.pending, tabId) })),
}))

function omit<T>(source: Record<string, T>, key: string): Record<string, T> {
  const next = { ...source }
  delete next[key]
  return next
}
