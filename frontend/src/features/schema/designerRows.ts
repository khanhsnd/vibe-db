import type { Cell } from '@/bridge/tables'
import { CHECK_ON, type ColumnEditor, type GridData } from '@/features/results/resultSet'

import { REFERENTIAL_ACTIONS } from './dataTypes'
import type { DraftField, DraftIndex, DraftKey } from './draft'
import { formatFieldType, parseFieldType } from './fieldType'

/**
 * The three lists of the designer as rows and columns, and the way an edited cell is written back
 * onto the thing it came from. They live apart from the component because they are the whole of
 * what a designer *pane* is — the component is the toolbar and which pane is in front.
 *
 * Every list is a `ResultGrid`, since that is the one component for rows and columns. The columns
 * are the parts an object is *declared* from rather than the strings a server reports back.
 */

/** The editable columns of the Fields grid, in the order they are drawn. */
export const FIELD = {
  name: 0,
  type: 1,
  length: 2,
  decimal: 3,
  notNull: 4,
  key: 5,
  default: 6,
} as const

const INDEX = { name: 0, columns: 1, unique: 2, method: 3 } as const

const KEY = {
  name: 0,
  columns: 1,
  referencedSchema: 2,
  referencedTable: 3,
  referencedColumns: 4,
  onUpdate: 5,
  onDelete: 6,
} as const

// Length and decimals read as 0 rather than blank when the type declares none, so a column of
// numbers stays a column of numbers.
export function fieldRows(draft: DraftField[], types: readonly string[]): GridData {
  const keys = keyPositions(draft)

  return {
    columns: [
      column('Name'),
      column('Type', 'text', { kind: 'choice', options: types }),
      column('Length', 'int'),
      column('Decimal', 'int'),
      column('Not null', 'text', { kind: 'check' }),
      column('Key', 'text', { kind: 'key' }),
      column('Default'),
    ],
    rows: draft.map((field, at): Cell[] => {
      const type = parseFieldType(field.type)
      return [
        field.name,
        type.base,
        String(type.length ?? 0),
        String(type.scale ?? 0),
        field.nullable && !field.primaryKey ? '' : CHECK_ON,
        keys[at] ?? '',
        field.default ?? '',
      ]
    }),
  }
}

export function indexRows(draft: DraftIndex[], methods: readonly string[]): GridData {
  return {
    columns: [
      column('Name'),
      column('Fields'),
      column('Unique', 'text', { kind: 'check' }),
      column('Method', 'text', { kind: 'choice', options: methods }),
    ],
    rows: draft.map((index): Cell[] => [
      index.name,
      joinNames(index.columns),
      index.unique ? CHECK_ON : '',
      index.method,
    ]),
  }
}

export function keyRows(draft: DraftKey[]): GridData {
  const action: ColumnEditor = { kind: 'choice', options: REFERENTIAL_ACTIONS }

  return {
    columns: [
      column('Name'),
      column('Fields'),
      column('Referenced Schema'),
      column('Referenced Table'),
      column('Referenced Fields'),
      column('On Update', 'text', action),
      column('On Delete', 'text', action),
    ],
    rows: draft.map((key): Cell[] => [
      key.name,
      joinNames(key.columns),
      key.referencedSchema,
      key.referencedTable,
      joinNames(key.referencedColumns),
      key.onUpdate,
      key.onDelete,
    ]),
  }
}

// writeField puts one edited cell back on the field it came from. Length and decimals are parts
// of the type rather than facts of their own, which is what they are on the wire.
export function writeField(field: DraftField, at: number, text: string): DraftField {
  const type = parseFieldType(field.type)
  const count = digitsOf(text)

  switch (at) {
    case FIELD.name:
      return { ...field, name: text }
    case FIELD.type:
      return { ...field, type: formatFieldType({ ...type, base: text }) }
    case FIELD.length:
      return { ...field, type: formatFieldType({ ...type, length: count }) }
    case FIELD.decimal:
      return { ...field, type: formatFieldType({ ...type, scale: count }) }
    case FIELD.notNull:
      return { ...field, nullable: !truthy(text) }
    case FIELD.key:
      return { ...field, primaryKey: truthy(text) }
    default:
      return { ...field, default: text === '' ? null : text }
  }
}

export function writeIndex(index: DraftIndex, at: number, text: string): DraftIndex {
  switch (at) {
    case INDEX.name:
      return { ...index, name: text }
    case INDEX.columns:
      return { ...index, columns: splitNames(text) }
    case INDEX.unique:
      return { ...index, unique: truthy(text) }
    default:
      return { ...index, method: text }
  }
}

export function writeKey(key: DraftKey, at: number, text: string): DraftKey {
  switch (at) {
    case KEY.name:
      return { ...key, name: text }
    case KEY.columns:
      return { ...key, columns: splitNames(text) }
    case KEY.referencedSchema:
      return { ...key, referencedSchema: text }
    case KEY.referencedTable:
      return { ...key, referencedTable: text }
    case KEY.referencedColumns:
      return { ...key, referencedColumns: splitNames(text) }
    case KEY.onUpdate:
      return { ...key, onUpdate: text }
    default:
      return { ...key, onDelete: text }
  }
}

/**
 * Whether this cell of an existing field differs from what the server holds. A field the server
 * has not seen is ringed whole instead — see `TableDesigner`.
 */
export function fieldEdited(field: DraftField, held: DraftField, at: number): boolean {
  const type = parseFieldType(field.type)
  const was = parseFieldType(held.type)

  switch (at) {
    case FIELD.name:
      return field.name !== held.name
    case FIELD.type:
      return type.base !== was.base
    case FIELD.length:
      return type.length !== was.length
    case FIELD.decimal:
      return type.scale !== was.scale
    case FIELD.notNull:
      return field.nullable !== held.nullable
    case FIELD.key:
      return field.primaryKey !== held.primaryKey
    default:
      return false
  }
}

// What Save will do, in the order it will do it. Not the SQL: how a statement is spelled is the
// driver's business, and this side has never seen one.
export function changeRows(described: string[]): GridData {
  return {
    columns: [column('#', 'int'), column('Change')],
    rows: described.map((describe, index): Cell[] => [String(index + 1), describe]),
  }
}

/**
 * What a yes/no cell means. A checkbox and a key cell write `CHECK_ON`, and a key cell reads back
 * its position in the key, so anything held at all is yes — bar a word that says otherwise, which
 * is all a paste can bring. A cleared cell is no.
 */
function truthy(text: string): boolean {
  const said = text.trim().toLowerCase()
  return said !== '' && said !== 'no' && said !== 'n' && said !== 'false' && said !== '0'
}

/** A list of columns is typed as one cell, so it is written and read the way it looks. */
function splitNames(text: string): string[] {
  return text
    .split(',')
    .map((name) => name.trim())
    .filter((name) => name !== '')
}

function joinNames(names: string[]): string {
  return names.join(', ')
}

function digitsOf(text: string): number | null {
  const digits = text.replace(/\D/g, '')
  return digits === '' ? null : Number(digits)
}

// Which column of the key each field is, counted down the rows: the number beside its lock. A
// field outside the key holds nothing, which is what leaves its cell empty.
function keyPositions(draft: DraftField[]): string[] {
  let taken = 0
  return draft.map((field) => (field.primaryKey ? String(++taken) : ''))
}

const column = (name: string, type = 'text', editor?: ColumnEditor) => ({
  name,
  type,
  fullType: type,
  editor,
})
