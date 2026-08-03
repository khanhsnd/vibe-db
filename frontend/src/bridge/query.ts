import * as api from '@wailsjs/go/app/QueryAPI'
import type { app } from '@wailsjs/go/models'

import { call } from './errors'
import type { Cell, TableColumn } from './tables'

export interface QueryRequest {
  runId: string
  database: string
  schema: string
  sql: string
  maxRows: number
  explain: boolean
}

/**
 * What one statement of a script did. `error` is the server's own wording and is empty for one
 * that succeeded — a script reports a rejection here rather than by failing, because the
 * statements before it did run.
 */
export interface StatementOutcome {
  sql: string
  rows: number
  rowsAffected: number
  elapsedMs: number
  error: string
  errorCode: string
}

/**
 * A statement that returns rows fills `columns`; one that does not reports `rowsAffected`.
 * The namespace and `table` name the source the rows can be written back to, and are empty
 * for a result the backend could not trace to a single table.
 *
 * `statements` is every statement the editor's text held, in the order they ran — one entry for
 * a single statement, and the whole script for one carrying several.
 */
export interface QueryResult {
  columns: TableColumn[]
  rows: Cell[][]
  rowsAffected: number
  truncated: boolean
  elapsedMs: number
  database: string
  schema: string
  table: string
  statements: StatementOutcome[]
}

/** The statement a script stopped at, if it stopped at one. */
export function failedStatement(result: QueryResult | null): StatementOutcome | null {
  return result?.statements.find((statement) => statement.error !== '') ?? null
}

export const queries = {
  run: (sessionId: string, request: QueryRequest): Promise<QueryResult> =>
    call(() =>
      api.Run(sessionId, request as unknown as app.QueryInput),
    ) as unknown as Promise<QueryResult>,

  cancel: (runId: string): Promise<void> => call(() => api.Cancel(runId)),
}
