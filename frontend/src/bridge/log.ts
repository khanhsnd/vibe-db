import * as api from '@wailsjs/go/app/QueryAPI'

import { call } from './errors'

/**
 * One statement the app has run, as the History Log lists it — one entry per *run*, where a
 * `HistoryEntry` is one per statement. `connection` is the profile's name rather than its id,
 * because a log is read long after the tab it was typed in has closed.
 *
 * `error` is the server's own wording and is empty for a statement that ran.
 */
export interface LogEntry {
  /**
   * Numbers the entries so a viewer already holding some asks for the rest. It counts up for as
   * long as the app runs and is never reused: past the cap entries fall off the front rather than
   * being renumbered, so a jump in the numbers is a viewer that looked away too long.
   */
  seq: number
  /** When it ran, in epoch milliseconds, for the viewer to draw in the reader's own time zone. */
  ranAt: number
  connection: string
  engine: string
  database: string
  schema: string
  sql: string
  elapsedMs: number
  error: string
}

export const statementLog = {
  /** Every statement run since the app started that is numbered past `after`, oldest first. */
  since: (after: number): Promise<LogEntry[]> =>
    call(() => api.ListLog(after)) as unknown as Promise<LogEntry[]>,

  /** Empties the log. Nothing a server did changes, and the query history is left alone. */
  clear: (): Promise<void> => call(() => api.ClearLog()),
}
