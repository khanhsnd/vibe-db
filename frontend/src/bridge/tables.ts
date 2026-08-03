import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'

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
  database: string
  schema: string
  table: string
  limit: number
  offset: number
  sort: TableSortRequest
}

export interface TablePage {
  columns: TableColumn[]
  rows: Cell[][]
  total: number
  limit: number
  offset: number
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

export const tables = {
  loadRows: (sessionId: string, page: TablePageRequest): Promise<TablePage> =>
    call(() =>
      api.LoadTableRows(sessionId, page as unknown as app.TablePageInput),
    ) as unknown as Promise<TablePage>,

  loadList: (sessionId: string, ns: Namespace): Promise<TableList> =>
    call(() => api.LoadTableList(sessionId, ns)) as unknown as Promise<TableList>,

  updateCell: (sessionId: string, edit: CellEdit): Promise<void> =>
    call(() => api.UpdateCell(sessionId, edit as unknown as app.CellEditInput)),

  insertRow: (sessionId: string, insert: RowInsert): Promise<void> =>
    call(() => api.InsertRow(sessionId, insert as unknown as app.RowInsertInput)),

  deleteRow: (sessionId: string, removal: RowDelete): Promise<void> =>
    call(() => api.DeleteRow(sessionId, removal as unknown as app.RowDeleteInput)),
}
