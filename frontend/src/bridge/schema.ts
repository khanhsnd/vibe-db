import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'

import type { Namespace } from './connections'
import { call } from './errors'
import type { Cell, TableColumn } from './tables'

export interface TableIndex {
  name: string
  columns: string[]
  unique: boolean
  primary: boolean
  method: string
}

export interface ForeignKey {
  name: string
  columns: string[]
  referencedSchema: string
  referencedTable: string
  referencedColumns: string[]
  onUpdate: string
  onDelete: string
}

/**
 * One piece a table's rows are stored in. `method` and `expression` are facts of the *table*
 * repeated on every one of its pieces — a list of pieces that did not say what it was cut by would
 * say very little — and `bounds` is the piece's own, in the engine's own words.
 */
export interface Partition {
  name: string
  method: string
  expression: string
  bounds: string
  /** What the server *estimates*, and `null` where it has no number. */
  rows: number | null
}

/** A partition as the reader declares it; `bounds` is their own clause and travels verbatim. */
export interface PartitionSpec {
  name: string
  bounds: string
}

/** A table's shape rather than its rows: what the tree draws under a table. */
export interface TableStructure {
  columns: TableColumn[]
  indexes: TableIndex[]
  foreignKeys: ForeignKey[]
  /**
   * Empty both for a table that is not partitioned and on an engine that does not partition at all
   * — see `hasPartitions`, which is what tells the two apart.
   */
  partitions: Partition[]
}

/**
 * A field as a dialog declares it. `type` carries its length and scale — `varchar(255)` — and
 * `default` is an expression, since that is what a server-side default is; `null` means the
 * field has none.
 */
export interface FieldSpec {
  name: string
  type: string
  nullable: boolean
  default: string | null
  primaryKey: boolean
  /**
   * What the field is to be described as on the server. `''` is a field with nothing written about
   * it, and setting it back to `''` is what clears a description — the rule an altered column
   * already follows for its default. It is ignored by an engine with nowhere to keep one.
   */
  comment: string
}

/** An index as the designer declares it; `method` empty takes the engine's own default. */
export interface IndexSpec {
  name: string
  columns: string[]
  unique: boolean
  method: string
}

/**
 * A referential constraint as the designer declares it. `referencedSchema` is whichever part of
 * a namespace the engine qualifies a table by, and is empty for a table in the same namespace.
 */
export interface ForeignKeySpec {
  name: string
  columns: string[]
  referencedSchema: string
  referencedTable: string
  referencedColumns: string[]
  onUpdate: string
  onDelete: string
}

/**
 * Which of the declared objects a definition belongs to. It is the same word the Go side checks
 * against before anything spells a `DROP` with it, so nothing else may be sent.
 *
 * The last three are the kinds only some engines have — see `hasMaterializedViews`, `hasSequences`
 * and `hasEvents`, which is what keeps a folder that could only ever be empty off the tree.
 */
export type SourceKind =
  | 'view'
  | 'function'
  | 'procedure'
  | 'trigger'
  | 'materializedView'
  | 'sequence'
  | 'event'

/**
 * One view, routine or trigger, named. The two fields beside the name are each one engine's way of
 * saying *which*: `arguments` is PostgreSQL's overload discriminator — two of its functions may
 * share a name and differ only in what they take — and `table` is the table a trigger fires on,
 * a trigger being declared and dropped about a table rather than on its own. Both are empty
 * wherever the name already identifies the object.
 */
export interface SourceRef {
  kind: SourceKind
  name: string
  arguments: string
  table: string
}

/** A definition as the server keeps it: the whole statement that declares the object. */
export interface Source extends SourceRef {
  definition: string
}

/** A reference to nothing, which is what an object being declared replaces. */
export const NEW_SOURCE: SourceRef = { kind: 'view', name: '', arguments: '', table: '' }

/**
 * What a routine does with one of its parameters. It is the same word the Go side checks against
 * before anything spells a call with it, so nothing else may be sent — and it decides which way the
 * value travels: `in` is written into the call and `out` is read back out of it.
 */
export type ParamMode = 'in' | 'out' | 'inout'

/**
 * One parameter a routine declares, as the catalogue keeps it. `type` is the engine's own spelling
 * and is what the reader is shown beside the box — it is never parsed here.
 */
export interface RoutineParam {
  name: string
  mode: ParamMode
  type: string
}

/**
 * One parameter with the value the reader gave it. `null` is the keyword `NULL` rather than the
 * word, and is what every `out` parameter carries: there is nothing to give one.
 */
export interface RoutineArg extends RoutineParam {
  value: string | null
}

/** The two kinds that are called. A view has no arguments and a trigger is fired by a write. */
export const CALLABLE_KINDS: readonly SourceKind[] = ['function', 'procedure']

/**
 * What a call answered with: the rows a function or a procedure returned, or the values its OUT
 * parameters hold, or the count of what it changed where it returned neither.
 *
 * It is not a query's result and carries no `statements`: what ran is what the preview already
 * showed, and a call is one thing the reader asked for however many statements the engine needs to
 * spell it.
 */
export interface CallResult {
  columns: TableColumn[]
  rows: Cell[][]
  rowsAffected: number
  elapsedMs: number
}

export function emptyFieldSpec(): FieldSpec {
  return { name: '', type: '', nullable: true, default: null, primaryKey: false, comment: '' }
}

/** The field a column already stored would be declared by, which is what an edit starts from. */
export function fieldSpecOf(column: TableColumn): FieldSpec {
  return {
    name: column.name,
    type: column.fullType || column.type,
    nullable: column.nullable,
    default: column.default,
    primaryKey: column.primaryKey,
    comment: column.comment ?? '',
  }
}

export const schema = {
  loadStructure: (sessionId: string, ns: Namespace, table: string): Promise<TableStructure> =>
    call(() => api.LoadTableStructure(sessionId, ns, table)) as unknown as Promise<TableStructure>,

  createTable: (sessionId: string, ns: Namespace, table: string, fields: FieldSpec[]) =>
    call(() =>
      api.CreateTable(sessionId, {
        ...ns,
        table,
        columns: fields.map(toColumnInput),
      } as unknown as app.TableSpecInput),
    ),

  renameTable: (sessionId: string, ns: Namespace, table: string, name: string) =>
    call(() => api.RenameTable(sessionId, ns, table, name)),

  /** A second table shaped like this one, holding its rows only when asked. */
  duplicateTable: (
    sessionId: string,
    ns: Namespace,
    table: string,
    name: string,
    withRows: boolean,
  ) => call(() => api.DuplicateTable(sessionId, ns, table, name, withRows)),

  dropTable: (sessionId: string, ns: Namespace, table: string) =>
    call(() => api.DropTable(sessionId, ns, table)),

  truncateTable: (sessionId: string, ns: Namespace, table: string) =>
    call(() => api.TruncateTable(sessionId, ns, table)),

  emptyTable: (sessionId: string, ns: Namespace, table: string) =>
    call(() => api.EmptyTable(sessionId, ns, table)),

  addField: (sessionId: string, ns: Namespace, table: string, field: FieldSpec) =>
    call(() => api.AddColumn(sessionId, ns, table, toColumnInput(field))),

  alterField: (sessionId: string, ns: Namespace, table: string, column: string, field: FieldSpec) =>
    call(() => api.AlterColumn(sessionId, ns, table, column, toColumnInput(field))),

  renameField: (sessionId: string, ns: Namespace, table: string, column: string, name: string) =>
    call(() => api.RenameColumn(sessionId, ns, table, column, name)),

  dropField: (sessionId: string, ns: Namespace, table: string, column: string) =>
    call(() => api.DropColumn(sessionId, ns, table, column)),

  /** Replaces the table's key with these columns; an empty list drops it. */
  setPrimaryKey: (sessionId: string, ns: Namespace, table: string, columns: string[]) =>
    call(() => api.SetPrimaryKey(sessionId, ns, table, columns)),

  createIndex: (sessionId: string, ns: Namespace, table: string, spec: IndexSpec) =>
    call(() => api.CreateIndex(sessionId, ns, table, spec as unknown as app.IndexInput)),

  dropIndex: (sessionId: string, ns: Namespace, table: string, index: string) =>
    call(() => api.DropIndex(sessionId, ns, table, index)),

  createForeignKey: (sessionId: string, ns: Namespace, table: string, spec: ForeignKeySpec) =>
    call(() => api.CreateForeignKey(sessionId, ns, table, spec as unknown as app.ForeignKeyInput)),

  dropForeignKey: (sessionId: string, ns: Namespace, table: string, constraint: string) =>
    call(() => api.DropForeignKey(sessionId, ns, table, constraint)),

  /** One more piece of a partitioned table; `bounds` is the reader's own clause. */
  addPartition: (sessionId: string, ns: Namespace, table: string, spec: PartitionSpec) =>
    call(() => api.AddPartition(sessionId, ns, table, spec as unknown as app.PartitionInput)),

  /** Removes the piece **and the rows in it** — which is what its confirmation has to say. */
  dropPartition: (sessionId: string, ns: Namespace, table: string, partition: string) =>
    call(() => api.DropPartition(sessionId, ns, table, partition)),

  /** Removes the rows and leaves the piece, the way Truncate does one level up. */
  truncatePartition: (sessionId: string, ns: Namespace, table: string, partition: string) =>
    call(() => api.TruncatePartition(sessionId, ns, table, partition)),

  /** Recomputes the rows a materialized view stores, which is the one thing only it needs. */
  refreshMaterializedView: (sessionId: string, ns: Namespace, name: string) =>
    call(() => api.RefreshMaterializedView(sessionId, ns, name)),

  /**
   * The whole reference, not the name: on PostgreSQL two functions may share one name and a
   * trigger is found by its table as well as its own.
   */
  dropSource: (sessionId: string, ns: Namespace, ref: SourceRef) =>
    call(() => api.DropSource(sessionId, ns, ref as unknown as app.SourceRef)),

  /**
   * The declaration a reader starts a *new* one of these from. It asks the server nothing — a
   * routine's body is grammar and so the engine's own — and answers with `''` where this engine
   * has no such object to start.
   */
  newSource: (sessionId: string, ns: Namespace, ref: SourceRef): Promise<string> =>
    call(() => api.NewSource(sessionId, ns, ref as unknown as app.SourceRef)) as Promise<string>,

  /** One view, routine or trigger as the statement that declares it — what the editor opens on. */
  loadSource: (sessionId: string, ns: Namespace, ref: SourceRef): Promise<Source> =>
    call(() =>
      api.LoadSource(sessionId, ns, ref as unknown as app.SourceRef),
    ) as unknown as Promise<Source>,

  /**
   * Exactly what `saveSource` will run. A definition being replaced is **dropped first**, so this
   * is the one place a reader sees that before it happens.
   */
  previewSource: (
    sessionId: string,
    ns: Namespace,
    replaces: SourceRef,
    definition: string,
  ): Promise<string[]> =>
    call(() =>
      api.PreviewSource(sessionId, ns, { replaces, definition } as unknown as app.SourceSaveInput),
    ) as unknown as Promise<string[]>,

  /** A blank name in `replaces` declares the object rather than replacing one. */
  saveSource: (sessionId: string, ns: Namespace, replaces: SourceRef, definition: string) =>
    call(() =>
      api.SaveSource(sessionId, ns, { replaces, definition } as unknown as app.SourceSaveInput),
    ),

  /**
   * What a routine takes, in the order it takes it — a catalogue read, so an engine with no stored
   * routines answers `UNSUPPORTED` naming it rather than an empty list.
   */
  routineParams: (sessionId: string, ns: Namespace, ref: SourceRef): Promise<RoutineParam[]> =>
    call(() =>
      api.LoadRoutineParams(sessionId, ns, ref as unknown as app.SourceRef),
    ) as unknown as Promise<RoutineParam[]>,

  /**
   * Exactly what `callRoutine` will run. It is the last thing a reader sees before a routine
   * touches their rows, which is why the pane draws it whether or not there is a parameter to fill
   * in — and why an engine with no grammar for the call answers with nothing rather than failing.
   */
  previewCall: (sessionId: string, ns: Namespace, ref: SourceRef, args: RoutineArg[]) =>
    call(() =>
      api.PreviewCall(sessionId, ns, {
        routine: ref,
        args,
      } as unknown as app.RoutineCallInput),
    ) as unknown as Promise<string[]>,

  /** Runs what `previewCall` showed, and answers with what the routine gave back. */
  callRoutine: (sessionId: string, ns: Namespace, ref: SourceRef, args: RoutineArg[]) =>
    call(() =>
      api.CallRoutine(sessionId, ns, {
        routine: ref,
        args,
      } as unknown as app.RoutineCallInput),
    ) as unknown as Promise<CallResult>,
}

// A field with no default leaves the key out rather than sending null, so Go reads a nil
// pointer and the statement carries no DEFAULT clause at all.
function toColumnInput(field: FieldSpec): app.ColumnInput {
  return {
    name: field.name,
    type: field.type,
    nullable: field.nullable,
    primaryKey: field.primaryKey,
    comment: field.comment,
    ...(field.default === null ? {} : { default: field.default }),
  } as app.ColumnInput
}
