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
  /**
   * What the server said it would do, filled only by a run that asked to be explained and null for
   * every other. Explain runs the statement like any other and asks about it as well, so the `rows`
   * above are the statement's own and the plan is *about* that run rather than instead of it.
   */
  plan: QueryPlan | null
}

/**
 * One step of a plan. `depth` is how deep in the plan's tree it sits and the steps arrive in the
 * order the tree reads, so the list is the whole tree — which is what draws it, `ResultGrid` having
 * rows and not branches. `type` is the engine's own name for the step and is never translated.
 */
export interface PlanNode {
  type: string
  detail: string
  depth: number
  rows: number
  cost: number
  selfCost: number
}

/** One thing the engine said about the plan as a whole, in its own wording. */
export interface PlanFact {
  label: string
  value: string
}

/**
 * A plan in the forms it is read in, the first of them being the engine's own: `columns` and `rows`
 * are what an EXPLAIN typed by hand answers with, and everything else is read out of that same
 * answer. `costed` says the engine puts numbers on a step at all — a document store does not, so its
 * plan is drawn without those columns rather than with columns full of zeroes.
 */
export interface QueryPlan {
  columns: TableColumn[]
  rows: Cell[][]
  nodes: PlanNode[]
  summary: PlanFact[]
  json: string
  costed: boolean
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

/**
 * A file of statements and where to run it. The path is one the reader picked in the host's own
 * dialog — see `queries.chooseFile` — and the encoding is how its bytes are read as text.
 */
export interface ScriptFileRequest {
  runId: string
  database: string
  schema: string
  path: string
  encoding: string
  /** Whether the rest of the file still runs after a statement the server refuses. */
  continueOnError: boolean
}

/** What a file did, one entry per statement in the order they ran. No rows are kept — see `RunFile`. */
export interface ScriptFileResult {
  statements: StatementOutcome[]
  failed: number
  elapsedMs: number
}

export interface EncodingInfo {
  value: string
  label: string
}

/**
 * How a file's bytes are read as text. The list mirrors the Go side's exactly — a name that is not
 * here is refused there rather than guessed at — and UTF-8 strips a byte order mark, which nearly
 * every file written by a Windows tool carries.
 */
export const SCRIPT_ENCODINGS: readonly EncodingInfo[] = [
  { value: 'utf-8', label: 'UTF-8' },
  { value: 'utf-16le', label: 'UTF-16 LE' },
  { value: 'utf-16be', label: 'UTF-16 BE' },
  { value: 'windows-1252', label: 'Windows-1252 (Western European)' },
  { value: 'windows-1258', label: 'Windows-1258 (Vietnamese)' },
  { value: 'iso-8859-1', label: 'ISO-8859-1 (Latin 1)' },
]

export const DEFAULT_ENCODING = 'utf-8'

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

  /** Raises the host's file picker. An empty path is a dismissed dialog, not a failure. */
  chooseFile: (): Promise<string> => call(() => api.ChooseScriptFile()),

  runFile: (sessionId: string, request: ScriptFileRequest): Promise<ScriptFileResult> =>
    call(() =>
      api.RunFile(sessionId, request as unknown as app.ScriptFileInput),
    ) as unknown as Promise<ScriptFileResult>,
}
