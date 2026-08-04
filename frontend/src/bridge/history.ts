import * as api from '@wailsjs/go/app/QueryAPI'

import { call } from './errors'

/**
 * One Execute the reader asked for, kept so it can be found and run again. `database` and `schema`
 * are where it ran, and travel back into the tab with it: a statement without its namespace may
 * name no table that exists.
 *
 * The outcome is the **last** run's, and `runs` is how many times the statement has been run — the
 * log keeps one entry per statement of a namespace rather than one per Execute. `error` is the
 * server's own wording and is empty for a run that finished.
 */
export interface HistoryEntry {
  profileId: string
  database: string
  schema: string
  sql: string
  rows: number
  rowsAffected: number
  elapsedMs: number
  errorCode: string
  error: string
  runs: number
  /** When it last ran, in epoch milliseconds, for the pane to draw in the reader's own time zone. */
  ranAt: number
}

export const queryHistory = {
  /** Every statement the app has run, newest first, across every connection. */
  list: (): Promise<HistoryEntry[]> =>
    call(() => api.ListHistory()) as unknown as Promise<HistoryEntry[]>,

  clear: (): Promise<void> => call(() => api.ClearHistory()),
}
