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
 * The rows to write and where they came from. They travel from the grid rather than being read
 * again, because what a reader asked to export is what they can see. `table` is empty for a
 * result no single table can be traced to, which only the SQL format has to make up a name for.
 */
export interface ExportRequest {
  namespace: Namespace
  table: string
  columns: string[]
  rows: Cell[][]
}

export const rowExport = {
  /** Answers with the path written, or an empty string for a dialog the reader dismissed. */
  run: (sessionId: string, format: ExportFormat, request: ExportRequest): Promise<string> =>
    call(() =>
      api.ExportRows(sessionId, {
        ...request.namespace,
        format,
        table: request.table,
        columns: request.columns,
        rows: request.rows,
      } as unknown as app.ExportInput),
    ),
}
