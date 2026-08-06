import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'

import type { Namespace } from './connections'
import { call } from './errors'

/**
 * The two wizards' half of the bridge: carrying a table out to a file, and reading one back in.
 *
 * It is not `export.ts`. That file is the *grid's* Export — the rows on screen, in one file the
 * reader names — and a dump, which is statements. This is a job over tables the reader never opened:
 * what the file is, where one field ends, which of its fields goes into which column, and what the
 * rows do to the ones already there.
 */

export type TransferFormat = app.TransferFormat
export type TransferEncoding = app.TransferEncoding
export type TransferMessage = app.TransferMessage
export type ImportPreview = app.ImportPreview
export type ImportReview = app.ImportReview
export type ImportResult = app.ImportResult
export type ExportJobResult = app.ExportJobResult
export type ExportedFile = app.ExportedFile

/**
 * Everything about a file that is not its bytes. Spelled out here rather than taken from
 * `app.TransferOptions` for the reason `ProfileInput` is: Wails generates a class for a wire type
 * held inside another, and an object literal can never satisfy one.
 */
export interface TransferOptions {
  encoding: string
  delimiter: string
  qualifier: string
  widths: number[]
  headerRow: boolean
  skip: number
  limit: number
  emptyIsNull: boolean
  null: string
  sheet: string
}

export interface ImportSource {
  format: string
  path: string
  options: TransferOptions
}

/** One of the file's fields pointed at one of the table's columns. */
export interface FieldMapping {
  /** The file's field, and `''` for a column the file has nothing for — left out of the write
   *  entirely, so the server's own default applies rather than a NULL. */
  source: string
  target: string
  /** Whether this column is part of what identifies a stored row. */
  key: boolean
}

export interface ImportRequest extends ImportSource {
  namespace: Namespace
  runId: string
  table: string
  mode: ImportMode
  mappings: FieldMapping[]
  batch: number
  continueOnError: boolean
}

export interface ExportRequest {
  namespace: Namespace
  runId: string
  format: string
  tables: string[]
  folder: string
  options: TransferOptions
  maxRows: number
}

/**
 * What an import does to the rows the table already holds. The list is closed and the Go side
 * mirrors it — `domain.ImportMode` — because what a mode is going to *do* decides whether it has to
 * be answered for first, and that rule is not re-derived here: see `transfer.reviewImport`.
 */
export type ImportMode = 'append' | 'update' | 'upsert' | 'skip' | 'delete' | 'copy'

export interface ImportModeInfo {
  value: ImportMode
  label: string
  /** Whether the mode finds stored rows first, and so cannot run without a key. */
  needsKey: boolean
}

export const IMPORT_MODES = [
  { value: 'append', label: 'Append — add every record to the destination', needsKey: false },
  {
    value: 'update',
    label: 'Update — change the records the key matches, and add none',
    needsKey: true,
  },
  {
    value: 'upsert',
    label: 'Append / Update — change the records the key matches, add the rest',
    needsKey: true,
  },
  {
    value: 'skip',
    label: 'Append without update — add what the key does not match, skip the rest',
    needsKey: true,
  },
  { value: 'delete', label: 'Delete — remove the records the key matches', needsKey: true },
  { value: 'copy', label: 'Copy — empty the destination, then add every record', needsKey: false },
] as const satisfies readonly ImportModeInfo[]

export function importMode(mode: string): ImportModeInfo {
  const [append] = IMPORT_MODES
  return IMPORT_MODES.find((one) => one.value === mode) ?? append
}

/** The whole of what a file's shape is, for a wizard that has not been told otherwise yet. */
export function defaultTransferOptions(): TransferOptions {
  return {
    encoding: 'utf-8',
    delimiter: ',',
    qualifier: '"',
    widths: [],
    headerRow: true,
    skip: 0,
    limit: 0,
    emptyIsNull: false,
    null: '',
    sheet: '',
  }
}

/** The delimiters worth a name. Anything else is typed, which is what `Other` is for. */
export const FIELD_DELIMITERS: ReadonlyArray<{ value: string; label: string }> = [
  { value: ',', label: 'Comma  ,' },
  { value: '\t', label: 'Tab' },
  { value: ';', label: 'Semicolon  ;' },
  { value: '|', label: 'Pipe  |' },
  { value: ' ', label: 'Space' },
]

export const TEXT_QUALIFIERS: ReadonlyArray<{ value: string; label: string }> = [
  { value: '"', label: 'Double quote  "' },
  { value: "'", label: "Single quote  '" },
  { value: '', label: 'None' },
]

const wire = <T>(value: unknown) => value as T

export const transfer = {
  listFormats: (): Promise<TransferFormat[]> => call(() => api.ListTransferFormats()),
  listEncodings: (): Promise<TransferEncoding[]> => call(() => api.ListTransferEncodings()),

  /** Answers with the path picked, or `''` for a dialog the reader dismissed. */
  chooseFile: (format: string): Promise<string> => call(() => api.ChooseTransferFile(format)),
  chooseFolder: (): Promise<string> => call(() => api.ChooseTransferFolder()),
  /** The sheets a workbook holds, and nothing at all for a file that is not one. */
  listWorksheets: (path: string): Promise<string[]> => call(() => api.ListWorksheets(path)),

  /** The head of the file, which is what the mapping step is drawn from. Needs no session. */
  preview: (source: ImportSource): Promise<ImportPreview> =>
    call(() => api.PreviewImportFile(wire<app.ImportSource>(source))),

  /**
   * What the import will do, and whether it has to be confirmed before it does it. The `confirm`
   * is worked out on the Go side from the mode and the connection's environment — see
   * `domain.Environment.NeedsConfirmation` — and is never re-derived here.
   */
  reviewImport: (sessionId: string, request: ImportRequest): Promise<ImportReview> =>
    call(() => api.ReviewImport(sessionId, importInput(request))),

  runImport: (sessionId: string, request: ImportRequest): Promise<ImportResult> =>
    call(() => api.RunImport(sessionId, importInput(request))),

  runExport: (sessionId: string, request: ExportRequest): Promise<ExportJobResult> =>
    call(() =>
      api.RunExport(
        sessionId,
        wire<app.ExportJobInput>({ ...request.namespace, ...request, namespace: undefined }),
      ),
    ),
}

function importInput(request: ImportRequest): app.ImportInput {
  return wire<app.ImportInput>({ ...request.namespace, ...request, namespace: undefined })
}
