import type { Namespace } from '@/bridge/connections'
import type { ForeignKey } from '@/bridge/schema'
import type { Cell } from '@/bridge/tables'
import type { Condition, Group } from '@/features/results/filter'
import type { ColumnReference } from '@/features/results/resultSet'

/** A foreign key seen from one of the columns it is declared on. */
export interface Reference {
  /** Where the referenced table is, which is this namespace unless the key says otherwise. */
  namespace: Namespace
  table: string
  /** Every pair the key holds: a column here, and the column there it is matched against. */
  pairs: ReadonlyArray<{ from: string; to: string }>
  /** The referenced column matched against *this* column, which is what a header names. */
  column: string
}

/**
 * The foreign keys of a table, by the column each is declared on. They are the catalogue's own —
 * the same list the tree draws under the table — so nothing here can follow a key that is not
 * there, and a column that merely *looks* like one (`user_id` with no constraint) gets no arrow.
 *
 * A column in two keys keeps the **first**, a header having room for one arrow; and a key whose
 * two lists disagree in length names no pairs worth trusting, so it is left out whole.
 */
export function referencesOf(
  namespace: Namespace,
  keys: readonly ForeignKey[],
): ReadonlyMap<string, Reference> {
  const found = new Map<string, Reference>()

  for (const key of keys) {
    if (key.referencedTable === '' || key.columns.length !== key.referencedColumns.length) continue

    const pairs = key.columns.map((from, at) => ({ from, to: key.referencedColumns[at] ?? '' }))

    for (const pair of pairs) {
      if (found.has(pair.from)) continue

      found.set(pair.from, {
        namespace: referencedNamespace(namespace, key.referencedSchema),
        table: key.referencedTable,
        pairs,
        column: pair.to,
      })
    }
  }

  return found
}

/**
 * The namespace a key points into. `referencedSchema` is whichever part of a namespace the engine
 * qualifies a table by — see `ForeignKeySpec` — so it replaces the schema where this namespace has
 * one and the database where it has not, and an empty one is this namespace as it stands.
 */
function referencedNamespace(namespace: Namespace, qualifier: string): Namespace {
  if (qualifier === '') return namespace

  return namespace.schema !== ''
    ? { ...namespace, schema: qualifier }
    : { ...namespace, database: qualifier }
}

/** Whether a key leaves the namespace it is declared in, which is when its table needs qualifying. */
const elsewhere = (from: Namespace, to: Namespace) =>
  from.database !== to.database || from.schema !== to.schema

/** The referenced table as a header says it: bare, unless it is in another namespace. */
export function referenceLabel(from: Namespace, reference: Reference): string {
  if (!elsewhere(from, reference.namespace)) return reference.table

  const qualifier = from.schema !== '' ? reference.namespace.schema : reference.namespace.database
  return `${qualifier}.${reference.table}`
}

/**
 * The conditions that name the row a referencing row points at: one `=` per pair of the key, read
 * off the row by column name. A **composite** key is therefore followed whole — one of its two
 * columns alone would name every row sharing it — and a member the row holds no value for (it is
 * NULL, or its column is hidden and so not on the grid) writes no condition rather than a wrong one.
 */
export function referenceFilter(reference: Reference, valueOf: (column: string) => Cell): Group {
  const children = reference.pairs.flatMap((pair): Condition[] => {
    const value = valueOf(pair.from)
    if (value === null) return []

    return [
      { kind: 'condition', column: pair.to, operator: 'eq', values: [value], enabled: true },
    ]
  })

  return { kind: 'group', conjunction: 'and', children, enabled: true }
}

/** What the grid is handed for one column: the reference, and the surface's way of opening it. */
export function columnReference(
  from: Namespace,
  reference: Reference,
  onOpen: ColumnReference['onOpen'],
): ColumnReference {
  return { table: referenceLabel(from, reference), column: reference.column, onOpen }
}
