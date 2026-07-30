import * as api from '@wailsjs/go/app/QueryAPI'
import type { app } from '@wailsjs/go/models'

import { call } from './errors'
import type { Cell, TableColumn } from './tables'

export const QUERY_ROW_LIMITS = [100, 1000, 10_000] as const
export const DEFAULT_QUERY_ROWS = 1000

export interface QueryRequest {
  runId: string
  database: string
  sql: string
  maxRows: number
  explain: boolean
}

/**
 * A statement that returns rows fills `columns`; one that does not reports `rowsAffected`.
 * `database` and `table` name the source the rows can be written back to, and are empty
 * for a result the backend could not trace to a single table.
 */
export interface QueryResult {
  columns: TableColumn[]
  rows: Cell[][]
  rowsAffected: number
  truncated: boolean
  elapsedMs: number
  database: string
  table: string
}

export const queries = {
  run: (sessionId: string, request: QueryRequest): Promise<QueryResult> =>
    call(() =>
      api.Run(sessionId, request as unknown as app.QueryInput),
    ) as unknown as Promise<QueryResult>,

  cancel: (runId: string): Promise<void> => call(() => api.Cancel(runId)),
}
