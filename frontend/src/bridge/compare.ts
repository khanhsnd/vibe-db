import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'

import type { Namespace } from './connections'
import { call } from './errors'
import type { Cell, TableColumn } from './tables'
import type { FieldSpec, ForeignKey, ForeignKeySpec, IndexSpec, TableIndex } from './schema'

/**
 * What a comparison says about one field, one index, one constraint or one row. The list is closed
 * and is the Go side's own wording — `added` is on the **right** and not on the left, `removed` is
 * the other way about — so nothing here re-derives which side is which.
 */
export type DiffStatus = 'same' | 'changed' | 'added' | 'removed'

/** Whether this is a difference at all, which is what the differences-only filter keeps. */
export function differs(status: DiffStatus): boolean {
  return status !== 'same'
}

/**
 * The facts a structure difference is reported in, spelled exactly as the Go side spells them —
 * `compare.diff*`. They are named here rather than matched as loose strings because what the
 * panes do with one is point at the *column* holding it, and a word that drifted would silently
 * stop pointing at anything.
 */
export const DIFFERENCE = {
  type: 'type',
  nullable: 'nullable',
  default: 'default',
  primaryKey: 'primary key',
  fields: 'fields',
  unique: 'unique',
  method: 'method',
  referencedSchema: 'referenced schema',
  referencedTable: 'referenced table',
  referencedFields: 'referenced fields',
  onUpdate: 'on update',
  onDelete: 'on delete',
} as const

/** One side of a comparison: which open session, which namespace, which table. */
export interface CompareSide {
  sessionId: string
  database: string
  schema: string
  table: string
}

export function compareSide(sessionId: string, ns: Namespace, table: string): CompareSide {
  return { sessionId, database: ns.database, schema: ns.schema, table }
}

/**
 * One thing of a structure comparison, whatever kind of thing it is. `left` or `right` is null for
 * one the other side alone holds, which is what the blank opposite it is drawn from, and `differs`
 * names the facts that changed — `type`, `nullable`, `fields` — so the reader is told which.
 */
interface Pair<T> {
  name: string
  status: DiffStatus
  differs: string[]
  left: T | null
  right: T | null
}

export type FieldDiff = Pair<TableColumn>
export type IndexDiff = Pair<TableIndex>
export type ForeignKeyDiff = Pair<ForeignKey>

/**
 * Two tables' shapes lined up, one entry per thing either side holds and in the left's own reading
 * order. Row *n* of the left pane and row *n* of the right pane are therefore the same field, which
 * is the whole of what makes two panes readable side by side.
 */
export interface StructureDiff {
  fields: FieldDiff[]
  indexes: IndexDiff[]
  foreignKeys: ForeignKeyDiff[]
}

/**
 * One row of a data comparison. `left` and `right` hold a value per compared column, in the order
 * `DataDiff.columns` names them, and one of the two is null for a row only the other side has.
 */
export interface RowDiff {
  status: DiffStatus
  key: string[]
  left: Cell[] | null
  right: Cell[] | null
  /** One flag per compared column, true where the sides hold different values. */
  differs: boolean[]
}

export interface DataDiff {
  /**
   * The columns rows were matched on, and whether they are the tables' own primary key rather than
   * the fallback. On the fallback a row is matched on the whole of itself, so nothing can be
   * `changed` — the toolbar says so, because "no changed rows" would otherwise read as a finding.
   */
  key: string[]
  keyDeclared: boolean
  /** The columns both sides hold, in the left's order, and the only ones compared. */
  columns: TableColumn[]
  /** The columns one side holds alone. Named rather than compared — that is a shape difference. */
  leftOnly: string[]
  rightOnly: string[]
  rows: RowDiff[]
  /** What each side's read returned, and whether the table held more than the compare asked for. */
  leftRows: number
  rightRows: number
  leftTruncated: boolean
  rightTruncated: boolean
}

/** How many rows each side reads when nothing else is picked, and what the picker offers. */
export const DEFAULT_COMPARE_ROWS = 1000
export const COMPARE_ROW_PRESETS = [100, 500, 1000, 5000] as const

/**
 * One change a migration makes to a table's shape. `kind` is a token from the closed list the Go
 * side keeps, and only the part belonging to that kind is read — the rest travel as blanks,
 * because JSON carries no union and one struct is what the wire has.
 *
 * The **same list** is sent to `preview` and to `apply`, which is the whole reason the script the
 * reader is shown cannot differ from what running it does.
 */
export type TableChangeKind =
  | 'addColumn'
  | 'alterColumn'
  | 'dropColumn'
  | 'createIndex'
  | 'dropIndex'
  | 'createForeignKey'
  | 'dropForeignKey'

export interface TableChange {
  kind: TableChangeKind
  /** What the change is addressed to — the column, index or constraint — empty for an add. */
  name: string
  column: FieldSpec
  index: IndexSpec
  key: ForeignKeySpec
}

/** A batch of rows and what they are to do to a table: the data half of a migration. */
export interface RowWrite {
  database: string
  schema: string
  table: string
  mode: 'append' | 'update' | 'upsert' | 'skip' | 'delete'
  columns: string[]
  /** The columns that identify a stored row. Every mode but an append needs them. */
  keys: string[]
  rows: Cell[][]
}

const wire = (side: CompareSide) => side as unknown as app.CompareSide

export const compare = {
  structure: (left: CompareSide, right: CompareSide): Promise<StructureDiff> =>
    call(() =>
      api.CompareStructure(wire(left), wire(right)),
    ) as unknown as Promise<StructureDiff>,

  data: (left: CompareSide, right: CompareSide, limit: number): Promise<DataDiff> =>
    call(() =>
      api.CompareData(wire(left), wire(right), limit),
    ) as unknown as Promise<DataDiff>,

  /** The table's own declaration, which is what a DDL diff reads both sides from. */
  createStatements: (sessionId: string, ns: Namespace, table: string): Promise<string[]> =>
    call(() => api.LoadCreateStatements(sessionId, ns, table)) as unknown as Promise<string[]>,

  /** What `applyChanges` would run, spelled by the engine and run by nothing. */
  previewChanges: (
    sessionId: string,
    ns: Namespace,
    table: string,
    changes: TableChange[],
  ): Promise<string[]> =>
    call(() =>
      api.PreviewTableChanges(sessionId, ns, table, changes as unknown as app.TableChangeInput[]),
    ) as unknown as Promise<string[]>,

  applyChanges: (sessionId: string, ns: Namespace, table: string, changes: TableChange[]) =>
    call(() =>
      api.ApplyTableChanges(sessionId, ns, table, changes as unknown as app.TableChangeInput[]),
    ),

  previewRows: (sessionId: string, write: RowWrite): Promise<string[]> =>
    call(() =>
      api.PreviewRowWrite(sessionId, write as unknown as app.RowWriteInput),
    ) as unknown as Promise<string[]>,

  applyRows: (sessionId: string, write: RowWrite) =>
    call(() => api.ApplyRowWrite(sessionId, write as unknown as app.RowWriteInput)),
}
