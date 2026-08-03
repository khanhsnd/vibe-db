import type { Namespace } from '@/bridge/connections'
import { emptyFieldSpec, schema, type FieldSpec, type TableStructure } from '@/bridge/schema'

/**
 * One field as the designer holds it while it is being worked on. `stored` is the name the field
 * answers to on the server, and is null for a field that is not there yet — that, rather than the
 * name in the boxes, is what a change is addressed to, so renaming and retyping in one sitting
 * still finds the column.
 */
export interface DraftField extends FieldSpec {
  stored: string | null
}

/** The fields of a table as they stand on the server: where a designer starts. */
export function draftOf(structure: TableStructure | undefined): DraftField[] {
  return (structure?.columns ?? []).map((column) => ({
    stored: column.name,
    name: column.name,
    type: column.fullType || column.type,
    nullable: column.nullable,
    default: column.default,
    primaryKey: column.primaryKey,
  }))
}

export function newField(): DraftField {
  return { ...emptyFieldSpec(), stored: null, type: 'text' }
}

/** A new table starts with the field one almost always starts with, already its key. */
export function newTableDraft(): DraftField[] {
  return [{ ...newField(), name: 'id', type: 'integer', nullable: false, primaryKey: true }]
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
 * What has to happen for the server's fields to become the draft's. The order matters: a column is
 * renamed before it is retyped, so the alter can name it; and the key is set last, once every
 * column it names exists.
 */
export function pendingChanges(draft: DraftField[], stored: DraftField[]): PendingChange[] {
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
    (field.default ?? '') !== (held.default ?? '')
  )
}
