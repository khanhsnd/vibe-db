import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'

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

export interface TablePageRequest {
  database: string
  schema: string
  table: string
  limit: number
  offset: number
}

export interface TablePage {
  columns: TableColumn[]
  rows: Cell[][]
  total: number
  limit: number
  offset: number
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
    call(() => api.LoadTableRows(sessionId, page)) as unknown as Promise<TablePage>,

  updateCell: (sessionId: string, edit: CellEdit): Promise<void> =>
    call(() => api.UpdateCell(sessionId, edit as unknown as app.CellEditInput)),

  insertRow: (sessionId: string, insert: RowInsert): Promise<void> =>
    call(() => api.InsertRow(sessionId, insert as unknown as app.RowInsertInput)),

  deleteRow: (sessionId: string, removal: RowDelete): Promise<void> =>
    call(() => api.DeleteRow(sessionId, removal as unknown as app.RowDeleteInput)),
}
