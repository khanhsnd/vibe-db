import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'

import type { Namespace } from './connections'
import { call } from './errors'
import type { Cell } from './tables'

export type ExportFormat = 'csv' | 'json' | 'sql'

/** The formats a grid offers, in the order the menu lists them. */
export const EXPORT_FORMATS: ReadonlyArray<{ value: ExportFormat; label: string }> = [
  { value: 'csv', label: 'Export as CSV…' },
  { value: 'json', label: 'Export as JSON…' },
  { value: 'sql', label: 'Export as SQL…' },
]

/**
 * The rows to write and what table they came from. They travel from the grid rather than being read
 * again, because what a reader asked to export is what they can see. `table` is empty for a
 * result no single table can be traced to, which only the SQL format has to make up a name for.
 *
 * There is no namespace: a SQL export names the table alone, so the file runs against whichever
 * database the connection is on.
 */
export interface ExportRequest {
  table: string
  columns: string[]
  rows: Cell[][]
}

export const rowExport = {
  /** Answers with the path written, or an empty string for a dialog the reader dismissed. */
  run: (sessionId: string, format: ExportFormat, request: ExportRequest): Promise<string> =>
    call(() =>
      api.ExportRows(sessionId, {
        format,
        table: request.table,
        columns: request.columns,
        rows: request.rows,
      } as unknown as app.ExportInput),
    ),
}

/**
 * What to dump and how much of it. It is not an export of a grid: nothing travels from the frontend
 * but the *names*, because a dump is read out of the server as it is written and is far larger than
 * anything a grid holds. Which file it lands in is asked for on the Go side, in the host's own save
 * dialog, and what kind of file it is belongs to the engine — SQL on a table store, the shell's own
 * script on a document store.
 */
export interface DumpRequest {
  namespace: Namespace
  /** The one table to dump, or `''` for every table the namespace holds. */
  table: string
  /** Whether the rows are written after the shape, which is the two items the menu offers. */
  withData: boolean
  /** Optional multi-namespace dump; each entry can narrow its own tables. */
  selections?: DumpSelection[]
}

export interface DumpSelection {
  namespace: Namespace
  tables: string[]
  all?: boolean
}

export const objectDump = {
  /** Answers with the path written, or an empty string for a dialog the reader dismissed. */
  run: (sessionId: string, request: DumpRequest): Promise<string> =>
    call(() =>
      api.DumpObjects(sessionId, {
        ...request.namespace,
        table: request.table,
        withData: request.withData,
        selections: request.selections?.map((selection) => ({
          ...selection.namespace,
          tables: selection.tables,
          all: selection.all ?? false,
        })),
      } as unknown as app.DumpInput),
    ),
}
