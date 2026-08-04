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

/**
 * How much of what a statement does cannot be taken back. It is read off the statement alone and
 * knows nothing about where it will run — pairing the two is the Go side's, and `confirm` below
 * is the answer it comes to.
 */
export type Hazard = '' | 'write' | 'unfiltered' | 'destructive'

/** One statement of the editor's text, as the reviewer reads it without running any of it. */
export interface StatementReview {
  /** Byte offsets into the editor's own text, so a mark lands on the statement that earned it. */
  start: number
  end: number
  sql: string
  keyword: string
  hazard: Hazard
  /**
   * Whether this has to be answered for before Execute may run it. It is the *answer*, worked out
   * on the Go side from the hazard and the connection's environment, so nothing here re-derives
   * a rule about what is dangerous.
   */
  confirm: boolean
}

/**
 * Something wrong with the script that could be proved without a server: a quote or a comment the
 * text ends inside of, a bracket with no partner. It is deliberately not a parser — whether a
 * column exists is the server's answer to give.
 */
export interface ScriptProblem {
  start: number
  end: number
  message: string
}

export interface ScriptReview {
  statements: StatementReview[]
  problems: ScriptProblem[]
  /** The connection's own, so the editor can say which one it is about to run against. */
  environment: string
}

/** A review of nothing, which is what a tab with no connection or no text has to show. */
export const NO_REVIEW: ScriptReview = { statements: [], problems: [], environment: '' }

/** The statements Execute has to ask about first, in the order they would run. */
export function needsConfirming(review: ScriptReview): StatementReview[] {
  return review.statements.filter((statement) => statement.confirm)
}

export const queries = {
  run: (sessionId: string, request: QueryRequest): Promise<QueryResult> =>
    call(() =>
      api.Run(sessionId, request as unknown as app.QueryInput),
    ) as unknown as Promise<QueryResult>,

  cancel: (runId: string): Promise<void> => call(() => api.Cancel(runId)),

  review: (sessionId: string, sql: string): Promise<ScriptReview> =>
    call(() => api.Review(sessionId, sql)) as unknown as Promise<ScriptReview>,

  format: (sessionId: string, sql: string): Promise<string> =>
    call(() => api.Format(sessionId, sql)),
}
