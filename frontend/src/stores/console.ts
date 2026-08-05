import { create } from 'zustand'

import type { Namespace } from '@/bridge/connections'
import type { AppError } from '@/bridge/errors'
import type { StatementOutcome } from '@/bridge/query'

import { useQueriesStore } from './queries'
import { useSettingsStore } from './settings'

/**
 * One thing the reader sent, and what came back. It is kept whatever became of it — a statement
 * the server refused is the one most likely to be wanted back — and it is what the transcript
 * above the prompt is drawn from.
 *
 * `statements` is per statement of what was sent, because one line may hold several. `error` is
 * the send's own failure: a single statement the server refused, or a connection that has gone.
 */
export interface ConsoleEntry {
  sql: string
  statements: StatementOutcome[]
  error: AppError | null
}

const NO_ENTRIES: ConsoleEntry[] = []

interface ConsoleState {
  entries: Record<string, ConsoleEntry[]>
  /**
   * Runs one line against the connection and adds it to the transcript. The run itself is the
   * query store's, so a console's rows are read, paged, exported and stopped exactly as a query
   * tab's are — this store holds only what a query tab keeps in its editor instead: the history.
   */
  send: (tabId: string, sessionId: string, namespace: Namespace, sql: string) => Promise<void>
  clear: (tabId: string) => void
  discard: (tabId: string) => void
}

export const entriesOf = (state: ConsoleState, tabId: string) => state.entries[tabId] ?? NO_ENTRIES

export const useConsoleStore = create<ConsoleState>((set) => ({
  entries: {},

  send: async (tabId, sessionId, namespace, sql) => {
    await useQueriesStore.getState().execute(tabId, sessionId, {
      database: namespace.database,
      schema: namespace.schema,
      sql,
      // A console has no Limit control of its own — it is a prompt, not a toolbar — so a line
      // typed at one reads as much as a fresh query tab would.
      maxRows: useSettingsStore.getState().settings.queryRows,
      explain: false,
    })

    const run = useQueriesStore.getState().runs[tabId]
    const entry: ConsoleEntry = {
      sql,
      statements: run?.result?.statements ?? [],
      error: run?.error ?? null,
    }

    set((s) => ({ entries: { ...s.entries, [tabId]: [...(s.entries[tabId] ?? []), entry] } }))
  },

  clear: (tabId) => set((s) => ({ entries: { ...s.entries, [tabId]: NO_ENTRIES } })),

  discard: (tabId) => set((s) => ({ entries: omit(s.entries, tabId) })),
}))

function omit<T>(source: Record<string, T>, key: string): Record<string, T> {
  const next = { ...source }
  delete next[key]
  return next
}
