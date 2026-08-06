import type {
  DataDiff,
  RowDiff,
  RowWrite,
  StructureDiff,
  TableChange,
} from '@/bridge/compare'
import { emptyFieldSpec, fieldSpecOf, type ForeignKeySpec, type IndexSpec } from '@/bridge/schema'
import type { Cell } from '@/bridge/tables'
import type { TableTarget } from '@/stores/tabs'


/**
 * Which way a migration runs. `toTarget` makes the right side look like the left, which is how the
 * tab is read: the left is what the reader has and the right is what they are comparing it against.
 * Turning it round swaps *which side is which* and nothing else — every rule below is written once
 * and takes the two sides as arguments.
 */
export type Direction = 'toTarget' | 'toOrigin'

/** The two ends of a migration, once the direction has said which is which. */
export interface Ends {
  /** Where the truth is taken from. */
  from: 'left' | 'right'
  /** Where it is written. This is the table that changes, and the only one that does. */
  into: 'left' | 'right'
}

export function endsOf(direction: Direction): Ends {
  return direction === 'toTarget' ? { from: 'left', into: 'right' } : { from: 'right', into: 'left' }
}

/** Which of the two tables a migration writes to, which is what Apply is addressed to. */
export function tableInto(direction: Direction, left: TableTarget, right: TableTarget): TableTarget {
  return direction === 'toTarget' ? right : left
}

const empty: IndexSpec = { name: '', columns: [], unique: false, method: '' }
const emptyKey: ForeignKeySpec = {
  name: '',
  columns: [],
  referencedSchema: '',
  referencedTable: '',
  referencedColumns: [],
  onUpdate: '',
  onDelete: '',
}

/** A change with the parts its kind does not read left blank, which is what the wire carries. */
function change(one: Partial<TableChange> & Pick<TableChange, 'kind'>): TableChange {
  return {
    name: '',
    column: emptyFieldSpec(),
    index: empty,
    key: emptyKey,
    ...one,
  }
}

/**
 * What has to happen to the table being written for the ticked things to match the other side.
 *
 * **The order is the point**, and it is the order the designer's own Save already uses: what is
 * going goes first, so a constraint never stands in the way of the field change under it; the
 * fields move next; and what is arriving is made last, so an index or a key can name a column this
 * same run has only just added.
 *
 * An index and a foreign key that merely *differ* become a drop and an add rather than an alter,
 * because neither engine can change which columns one covers — the same reason the designer
 * remakes them.
 */
export function structureChanges(
  diff: StructureDiff,
  ends: Ends,
  taken: ReadonlySet<string>,
): TableChange[] {
  const keys = pairChanges(diff.foreignKeys, ends, taken, SECTION.key, (one) => ({
    add: change({ kind: 'createForeignKey', key: one }),
    drop: change({ kind: 'dropForeignKey', name: one.name }),
  }))

  const indexes = pairChanges(diff.indexes, ends, taken, SECTION.index, (one) => ({
    add: change({ kind: 'createIndex', index: one }),
    drop: change({ kind: 'dropIndex', name: one.name }),
  }))

  const fields = pairChanges(diff.fields, ends, taken, SECTION.field, (one, held) => ({
    add: change({ kind: 'addColumn', column: fieldSpecOf(one) }),
    drop: change({ kind: 'dropColumn', name: one.name }),
    // A column that is on both sides is altered in place, which is the one thing of the three
    // that is a change rather than a drop and an add: `held` is what the target calls it now.
    alter: change({
      kind: 'alterColumn',
      name: held?.name ?? one.name,
      column: fieldSpecOf(one),
    }),
  }))

  return [...keys.drops, ...indexes.drops, ...fields.all, ...indexes.adds, ...keys.adds]
}

/** One pair of a structure comparison, whatever kind of thing it is about. */
interface Pair<T> {
  name: string
  status: string
  left: T | null
  right: T | null
}

/**
 * The drops and adds one list of pairs contributes, in the direction asked for. A pair only the
 * *source* holds is added, one only the *target* holds is dropped, and one both hold and differ
 * about is either altered where that is possible or dropped and remade where it is not.
 */
function pairChanges<T extends { name: string }>(
  pairs: readonly Pair<T>[],
  ends: Ends,
  taken: ReadonlySet<string>,
  kind: string,
  spell: (source: T, held: T | null) => { add: TableChange; drop: TableChange; alter?: TableChange },
): { drops: TableChange[]; adds: TableChange[]; all: TableChange[] } {
  const drops: TableChange[] = []
  const adds: TableChange[] = []
  const all: TableChange[] = []

  for (const pair of pairs) {
    if (pair.status === 'same' || !taken.has(pickKey(kind, pair.name))) continue

    const source = pair[ends.from]
    const held = pair[ends.into]

    if (source === null && held !== null) {
      const { drop } = spell(held, held)
      drops.push(drop)
      all.push(drop)
      continue
    }
    if (source === null) continue

    const { add, alter } = spell(source, held)
    if (held !== null && alter) {
      all.push(alter)
      continue
    }
    if (held !== null) {
      const { drop } = spell(held, held)
      drops.push(drop)
      all.push(drop)
    }
    adds.push(add)
    all.push(add)
  }

  return { drops, adds, all }
}

/**
 * How a ticked thing is named, so that a field and an index sharing a name are two ticks and not
 * one. It is the pair's own kind and name, which is what the grid draws a row per.
 */
export function pickKey(kind: string, name: string): string {
  return `${kind}:${name}`
}

/** Every pair of a comparison that could be ticked at all: the ones that differ. */
export function everyPick(diff: StructureDiff): string[] {
  return [
    ...pickable(diff.fields, 'field'),
    ...pickable(diff.indexes, 'index'),
    ...pickable(diff.foreignKeys, 'key'),
  ]
}

function pickable(pairs: readonly Pair<unknown>[], kind: string): string[] {
  return pairs.flatMap((pair) => (pair.status === 'same' ? [] : [pickKey(kind, pair.name)]))
}

/**
 * The two writes the data half of a migration is: everything the source holds and the target does
 * not agree with goes across as an **upsert** — changing the rows the key names and adding the ones
 * it does not — and everything only the *target* holds is **deleted**.
 *
 * They are two writes and not one because they are two intentions, and because `RowStatements`
 * takes one mode: a row that has to arrive and a row that has to go cannot be the same statement.
 * An empty one is left out rather than sent, so an Apply that only adds runs only inserts.
 */
export function dataWrites(
  diff: DataDiff,
  ends: Ends,
  taken: ReadonlySet<string>,
  into: TableTarget,
): RowWrite[] {
  const carried: Cell[][] = []
  const removed: Cell[][] = []

  diff.rows.forEach((row, at) => {
    if (row.status === 'same' || !taken.has(rowKey(row, at))) return

    const source = row[ends.from]
    const held = row[ends.into]

    if (source !== null) carried.push(source)
    else if (held !== null) removed.push(held)
  })

  const write = (mode: RowWrite['mode'], rows: Cell[][]): RowWrite[] =>
    rows.length === 0
      ? []
      : [
          {
            database: into.namespace.database,
            schema: into.namespace.schema,
            table: into.table,
            mode,
            columns: diff.columns.map((column) => column.name),
            keys: diff.key,
            rows,
          },
        ]

  return [...write('upsert', carried), ...write('delete', removed)]
}

/**
 * How a ticked row is named. It is the row's **position** in the comparison and not its key,
 * because a comparison of two tables with no key of their own matches rows on the whole of
 * themselves and two of them may read identically — a tick has to mean one row.
 */
export function rowKey(_row: RowDiff, at: number): string {
  return `row:${at}`
}

/** Every row of a data comparison that could be ticked: the ones that differ. */
export function everyRowPick(diff: DataDiff): string[] {
  return diff.rows.flatMap((row, at) => (row.status === 'same' ? [] : [rowKey(row, at)]))
}

/** The three lists a structure comparison ticks, so a caller can name one without spelling it. */
export const SECTION = { field: 'field', index: 'index', key: 'key' } as const
