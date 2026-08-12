import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'

import type { Namespace } from './connections'
import { call } from './errors'
import type { TableColumn } from './tables'

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

/** A table's shape rather than its rows: what the tree draws under a table. */
export interface TableStructure {
  columns: TableColumn[]
  indexes: TableIndex[]
  foreignKeys: ForeignKey[]
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
 * Which of the four declared objects a definition belongs to. It is the same word the Go side
 * checks against before anything spells a `DROP` with it, so nothing else may be sent.
 */
export type SourceKind = 'view' | 'function' | 'procedure' | 'trigger'

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

export function emptyFieldSpec(): FieldSpec {
  return { name: '', type: '', nullable: true, default: null, primaryKey: false }
}

/** The field a column already stored would be declared by, which is what an edit starts from. */
export function fieldSpecOf(column: TableColumn): FieldSpec {
  return {
    name: column.name,
    type: column.fullType || column.type,
    nullable: column.nullable,
    default: column.default,
    primaryKey: column.primaryKey,
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
}

// A field with no default leaves the key out rather than sending null, so Go reads a nil
// pointer and the statement carries no DEFAULT clause at all.
function toColumnInput(field: FieldSpec): app.ColumnInput {
  return {
    name: field.name,
    type: field.type,
    nullable: field.nullable,
    primaryKey: field.primaryKey,
    ...(field.default === null ? {} : { default: field.default }),
  } as app.ColumnInput
}
