import type { Namespace } from '@/bridge/connections'
import {
  emptyFieldSpec,
  schema,
  type FieldSpec,
  type ForeignKeySpec,
  type IndexSpec,
  type TableStructure,
} from '@/bridge/schema'

/**
 * One field as the designer holds it while it is being worked on. `stored` is the name the field
 * answers to on the server, and is null for a field that is not there yet — that, rather than the
 * name in the boxes, is what a change is addressed to, so renaming and retyping in one sitting
 * still finds the column.
 */
export interface DraftField extends FieldSpec {
  stored: string | null
}

/** Everything the designer stages: what Save reconciles against the table on the server. */
export interface TableDraft {
  fields: DraftField[]
  indexes: DraftIndex[]
  keys: DraftKey[]
}

/** A table as it stands on the server: where a designer starts, and what it is measured against. */
export function draftOf(structure: TableStructure | undefined): TableDraft {
  return {
    fields: (structure?.columns ?? []).map((column) => ({
      stored: column.name,
      name: column.name,
      type: column.fullType || column.type,
      nullable: column.nullable,
      default: column.default,
      primaryKey: column.primaryKey,
      comment: column.comment ?? '',
    })),
    indexes: indexDraftOf(structure),
    keys: keyDraftOf(structure),
  }
}

export function emptyDraft(): TableDraft {
  return { fields: [], indexes: [], keys: [] }
}

export function newField(): DraftField {
  return { ...emptyFieldSpec(), stored: null, type: 'text' }
}

/** A new table starts with the field one almost always starts with, already its key. */
export function newTableDraft(): TableDraft {
  return {
    fields: [{ ...newField(), name: 'id', type: 'integer', nullable: false, primaryKey: true }],
    indexes: [],
    keys: [],
  }
}

/**
 * An index and a foreign key are staged the way a field is, and for the same reason: `stored` is
 * what the server calls it, and null for one it has never seen. Neither engine can alter which
 * columns an index covers, so an edited one is dropped and made again — which is what makes one
 * `stored` name enough to address it.
 */
export interface DraftIndex extends IndexSpec {
  stored: string | null
}

export interface DraftKey extends ForeignKeySpec {
  stored: string | null
}

/**
 * The indexes of a table as they stand on the server. The one behind the primary key is left
 * out: it is the key, and it is the Fields grid that moves a key — see `pendingChanges`.
 */
export function indexDraftOf(structure: TableStructure | undefined): DraftIndex[] {
  return (structure?.indexes ?? [])
    .filter((index) => !index.primary)
    .map((index) => ({
      stored: index.name,
      name: index.name,
      columns: [...index.columns],
      unique: index.unique,
      method: index.method,
    }))
}

export function keyDraftOf(structure: TableStructure | undefined): DraftKey[] {
  return (structure?.foreignKeys ?? []).map((key) => ({
    stored: key.name,
    name: key.name,
    columns: [...key.columns],
    referencedSchema: key.referencedSchema,
    referencedTable: key.referencedTable,
    referencedColumns: [...key.referencedColumns],
    onUpdate: key.onUpdate,
    onDelete: key.onDelete,
  }))
}

export function newIndex(): DraftIndex {
  return { stored: null, name: '', columns: [], unique: false, method: '' }
}

export function newKey(): DraftKey {
  return {
    stored: null,
    name: '',
    columns: [],
    referencedSchema: '',
    referencedTable: '',
    referencedColumns: [],
    onUpdate: '',
    onDelete: '',
  }
}

/**
 * One thing Save will do, in the order it will be done. `describe` is what the Changes pane shows —
 * a plain reading of the change rather than the statement, because the statement is the driver's
 * and only it knows how the engine spells it.
 */
export interface PendingChange {
  describe: string
  run: (sessionId: string, ns: Namespace, table: string) => Promise<void>
}

/**
 * What has to happen for the table on the server to become the one on screen.
 *
 * The order is the point. What is going goes **first**, so a constraint never stands in the way
 * of the field change that follows it; the fields move next; and what is arriving is made
 * **last**, so an index or a key can name a column this same Save has only just added.
 */
export function pendingChanges(draft: TableDraft, stored: TableDraft): PendingChange[] {
  const indexes = stagedChanges(draft.indexes, stored.indexes, INDEX_ACTIONS)
  const keys = stagedChanges(draft.keys, stored.keys, KEY_ACTIONS)

  return [
    ...keys.drops,
    ...indexes.drops,
    ...fieldChanges(draft.fields, stored.fields),
    ...indexes.adds,
    ...keys.adds,
  ]
}

/**
 * What a table that has just been created still needs. `CREATE TABLE` carries its fields and its
 * primary key and nothing else, so anything staged in the other two lists is made afterwards.
 */
export function newTableConstraints(draft: TableDraft): PendingChange[] {
  return [
    ...stagedChanges(draft.indexes, [], INDEX_ACTIONS).adds,
    ...stagedChanges(draft.keys, [], KEY_ACTIONS).adds,
  ]
}

/** What a staged list can be told about itself: whether two of its entries are the same, and how
 * one is dropped or made. It is the whole of the difference between an index and a foreign key,
 * which are otherwise reconciled the same way. */
interface StagedActions<T> {
  same: (staged: T, held: T) => boolean
  drop: (name: string) => PendingChange
  add: (staged: T) => PendingChange
}

/**
 * Neither engine can alter which columns an index or a key covers, so anything that changed is
 * dropped and made again rather than altered — which is why one `stored` name is enough to
 * address either.
 */
function stagedChanges<T extends { stored: string | null; name: string }>(
  draft: T[],
  stored: T[],
  on: StagedActions<T>,
): { drops: PendingChange[]; adds: PendingChange[] } {
  const byStored = new Map(draft.filter((one) => one.stored !== null).map((one) => [one.stored, one]))
  const drops: PendingChange[] = []
  const adds: PendingChange[] = []

  for (const held of stored) {
    const kept = byStored.get(held.stored)
    if (!kept || !on.same(kept, held)) drops.push(on.drop(held.stored ?? held.name))
  }

  for (const staged of draft) {
    const held = stored.find((one) => one.stored !== null && one.stored === staged.stored)
    if (!held || !on.same(staged, held)) adds.push(on.add(staged))
  }

  return { drops, adds }
}

/** Whether two staged entries are the same object declared the same way — what the grid rings a
 * row by, and what decides whether Save drops and remakes it. */
export const sameIndex = (staged: DraftIndex, held: DraftIndex): boolean =>
  INDEX_ACTIONS.same(staged, held)

export const sameKey = (staged: DraftKey, held: DraftKey): boolean => KEY_ACTIONS.same(staged, held)

const INDEX_ACTIONS: StagedActions<DraftIndex> = {
  same: (staged, held) =>
    staged.name === held.name &&
    staged.unique === held.unique &&
    staged.method === held.method &&
    staged.columns.join() === held.columns.join(),

  drop: (name) => ({
    describe: `Drop index ${name}`,
    run: (sessionId, ns, table) => schema.dropIndex(sessionId, ns, table, name),
  }),

  add: (staged) => ({
    describe: `Add index ${staged.name} on ${staged.columns.join(', ')}`,
    run: (sessionId, ns, table) => schema.createIndex(sessionId, ns, table, staged),
  }),
}

const KEY_ACTIONS: StagedActions<DraftKey> = {
  same: (staged, held) =>
    staged.name === held.name &&
    staged.referencedSchema === held.referencedSchema &&
    staged.referencedTable === held.referencedTable &&
    staged.onUpdate === held.onUpdate &&
    staged.onDelete === held.onDelete &&
    staged.columns.join() === held.columns.join() &&
    staged.referencedColumns.join() === held.referencedColumns.join(),

  drop: (name) => ({
    describe: `Drop foreign key ${name}`,
    run: (sessionId, ns, table) => schema.dropForeignKey(sessionId, ns, table, name),
  }),

  add: (staged) => ({
    describe: `Add foreign key ${staged.name} → ${staged.referencedTable}`,
    run: (sessionId, ns, table) => schema.createForeignKey(sessionId, ns, table, staged),
  }),
}

/**
 * The field half. A column is renamed before it is retyped, so the alter can name it; and the
 * key is set last, once every column it names exists.
 */
function fieldChanges(draft: DraftField[], stored: DraftField[]): PendingChange[] {
  const changes: PendingChange[] = []
  const byStored = new Map(draft.filter((f) => f.stored !== null).map((f) => [f.stored, f]))

  for (const held of stored) {
    if (byStored.has(held.stored)) continue
    const column = held.stored ?? held.name
    changes.push({
      describe: `Drop field ${column}`,
      run: (sessionId, ns, table) => schema.dropField(sessionId, ns, table, column),
    })
  }

  for (const field of draft) {
    if (field.stored === null) {
      changes.push({
        describe: `Add field ${field.name} ${field.type}`,
        run: (sessionId, ns, table) => schema.addField(sessionId, ns, table, field),
      })
      continue
    }

    const held = stored.find((f) => f.stored === field.stored)
    if (!held) continue

    const column = field.stored
    if (field.name !== held.name) {
      changes.push({
        describe: `Rename field ${column} to ${field.name}`,
        run: (sessionId, ns, table) => schema.renameField(sessionId, ns, table, column, field.name),
      })
    }
    if (retyped(field, held)) {
      changes.push({
        // The column is named by what it will be called, since the rename above runs first.
        describe: `Alter field ${field.name} ${field.type}`,
        run: (sessionId, ns, table) => schema.alterField(sessionId, ns, table, field.name, field),
      })
    }
  }

  const keys = draft.filter((f) => f.primaryKey).map((f) => f.name)
  const held = stored.filter((f) => f.primaryKey).map((f) => f.name)
  if (keys.join() !== held.join()) {
    changes.push({
      describe: keys.length === 0 ? 'Drop the primary key' : `Primary key on ${keys.join(', ')}`,
      run: (sessionId, ns, table) => schema.setPrimaryKey(sessionId, ns, table, keys),
    })
  }

  return changes
}

// A key column is written NOT NULL whatever it claimed, so a column that only gained the key has
// not been retyped — the key change alone will carry it.
function retyped(field: DraftField, held: DraftField): boolean {
  return (
    field.type !== held.type ||
    (field.nullable && !field.primaryKey) !== (held.nullable && !held.primaryKey) ||
    (field.default ?? '') !== (held.default ?? '') ||
    field.comment !== held.comment
  )
}
