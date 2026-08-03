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

  dropIndex: (sessionId: string, ns: Namespace, table: string, index: string) =>
    call(() => api.DropIndex(sessionId, ns, table, index)),

  dropForeignKey: (sessionId: string, ns: Namespace, table: string, constraint: string) =>
    call(() => api.DropForeignKey(sessionId, ns, table, constraint)),
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
