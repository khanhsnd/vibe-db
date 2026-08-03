import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'
import { EventsOn } from '@wailsjs/runtime/runtime'

import type { Namespace } from './connections'
import { call } from './errors'

export type Cell = string | null

export interface TableColumn {
  name: string
  type: string
  fullType: string
  nullable: boolean
  primaryKey: boolean
  default: Cell
}

/** The column the read is ordered by; an empty name leaves the table's primary key in force. */
export interface TableSortRequest {
  column: string
  descending: boolean
}

export interface TablePageRequest {
  /**
   * Minted per read, so it can be stopped before it answers and so the batches of a streamed
   * read can be told from another tab's — see `onRowBatch`.
   */
  runId: string
  database: string
  schema: string
  table: string
  limit: number
  offset: number
  sort: TableSortRequest
}

/** A read that asked for `ALL_ROWS` answers with its columns and total, and no rows at all. */
export interface TablePage {
  columns: TableColumn[]
  rows: Cell[][]
  total: number
  limit: number
  offset: number
  /** The read that produced these rows, exactly as it ran, for the status line to show and copy. */
  statement: string
}

/** What a streamed read pushes as it goes — see `onRowsStreamed` for why they are two things. */
interface RowsBatch {
  runId: string
  rows: Cell[][]
}

interface RowsCounted {
  runId: string
  fetched: number
}

const ROWS_EVENT = 'rows:batch'
const COUNTED_EVENT = 'rows:counted'

/** How a caller follows one streamed read: its rows as they arrive, and how far it has got. */
export interface RowsStream {
  rows: (batch: Cell[][]) => void
  counted: (fetched: number) => void
}

/**
 * One row per table of a namespace: its name in the first column, then whatever else the
 * engine's catalogue keeps about it. The columns are the engine's own — a size or a date arrives
 * spelled the way the server spells it — so nothing here names or formats one.
 */
export interface TableList {
  columns: TableColumn[]
  rows: Cell[][]
}

export interface CellEdit {
  database: string
  schema: string
  table: string
  column: string
  value: Cell
  key: Record<string, string>
}

export interface RowValue {
  column: string
  value: Cell
}

/** `values` lists only the columns filled in; an omitted one takes the server's default. */
export interface RowInsert {
  database: string
  schema: string
  table: string
  values: RowValue[]
}

export interface RowDelete {
  database: string
  schema: string
  table: string
  key: Record<string, string>
}

/**
 * Follows one streamed read, and answers with how to stop following it. Every row of a large table
 * cannot cross the bridge in one piece — a call's result arrives as a single JavaScript string — so
 * a read of `ALL_ROWS` answers with its columns alone and pushes the rest here until the call
 * itself resolves.
 *
 * The rows and the count arrive **separately**, and that is the point: rows come in batches, which
 * land on round counts, while the count comes on a clock of its own. A reader watching the number
 * therefore sees what the read has truly reached, climbing, rather than a figure stepping in
 * thousands. See `app.EventRows` and `app.EventRowsCounted`.
 */
export function onRowsStreamed(runId: string, follow: RowsStream): () => void {
  const offRows = EventsOn(ROWS_EVENT, (batch: RowsBatch) => {
    if (batch.runId === runId) follow.rows(batch.rows)
  })
  const offCounted = EventsOn(COUNTED_EVENT, (count: RowsCounted) => {
    if (count.runId === runId) follow.counted(count.fetched)
  })

  return () => {
    offRows()
    offCounted()
  }
}

export const tables = {
  loadRows: (sessionId: string, page: TablePageRequest): Promise<TablePage> =>
    call(() =>
      api.LoadTableRows(sessionId, page as unknown as app.TablePageInput),
    ) as unknown as Promise<TablePage>,

  /** Stops a read before it answers. An id the backend has forgotten is ignored. */
  stopRows: (runId: string): Promise<void> => call(() => api.StopTableRows(runId)),

  loadList: (sessionId: string, ns: Namespace): Promise<TableList> =>
    call(() => api.LoadTableList(sessionId, ns)) as unknown as Promise<TableList>,

  updateCell: (sessionId: string, edit: CellEdit): Promise<void> =>
    call(() => api.UpdateCell(sessionId, edit as unknown as app.CellEditInput)),

  insertRow: (sessionId: string, insert: RowInsert): Promise<void> =>
    call(() => api.InsertRow(sessionId, insert as unknown as app.RowInsertInput)),

  deleteRow: (sessionId: string, removal: RowDelete): Promise<void> =>
    call(() => api.DeleteRow(sessionId, removal as unknown as app.RowDeleteInput)),
}
