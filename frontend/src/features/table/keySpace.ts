import type { Cell, TableColumn } from '@/bridge/tables'
import { NO_FILTER, type Condition, type Group } from '@/features/results/filter'

/**
 * The columns of a key space, by name. Naming them here is not the engine comparison the rules
 * forbid: a key space's five columns are a **constant** — the same five on every Redis server there
 * has ever been, which is what it has instead of a catalogue (see grid.md, *Where the columns come
 * from*) — and everything in this folder that reads one is reached only where `EngineInfo.keys` says
 * the namespace is a key space at all. What is never done is asking an engine's *name* anything.
 */
export const KEY_COLUMN = 'Key'
export const TYPE_COLUMN = 'Type'
export const VALUE_COLUMN = 'Value'
export const TTL_COLUMN = 'TTL'

/**
 * The six types a key can be, in the server's own spellings, because `TYPE` answers with these and a
 * reader who knows Redis knows these words. It is a fixed list rather than a read of the column's
 * values: a server holding no hash today can still be asked for hashes, and a list of six needs no
 * round trip.
 */
export const KEY_TYPES: readonly string[] = ['string', 'list', 'set', 'hash', 'zset', 'stream']

/** Which column of a read identifies a row: the key, and the only thing here that can. */
export function columnAt(columns: readonly TableColumn[], name: string): number {
  return columns.findIndex((column) => column.name === name)
}

export function cellAt(row: readonly Cell[] | undefined, index: number): Cell {
  return index < 0 ? null : (row?.[index] ?? null)
}

/**
 * What the reader has narrowed a key space to: a fragment of a key's name, and one of the six types.
 *
 * A key space is searched rather than filtered, and this is the whole of what the bar over it holds.
 * It is **the same conditions the builder would have written** — `Key contains …` and `Type = …` —
 * because the driver already answers exactly those two: it hands the first to the server as the one
 * narrowing Redis has (`SCAN … MATCH *text*`) and applies the second to what comes back. Spelling
 * them as two boxes rather than as a tree is a shorter way of saying the same thing, not a second
 * mechanism, which is why the panel can simply be left out.
 */
export interface KeySearch {
  text: string
  type: string
}

export const NO_SEARCH: KeySearch = { text: '', type: '' }

/** What the boxes show: the two conditions read back out of what is actually in force. */
export function readSearch(criteria: Group): KeySearch {
  const one = (column: string, operator: string) => {
    for (const child of criteria.children) {
      if (child.kind !== 'condition') continue
      if (child.column === column && child.operator === operator) return child.values[0] ?? ''
    }
    return ''
  }

  return { text: one(KEY_COLUMN, 'contains'), type: one(TYPE_COLUMN, 'eq') }
}

/**
 * The two boxes as the tree the read is narrowed by. An empty box says nothing rather than saying
 * `contains ''`, which every key would satisfy — and a search of neither is `NO_FILTER`, the tree a
 * table nobody has narrowed carries.
 */
export function writeSearch(search: KeySearch): Group {
  const children: Condition[] = []

  if (search.text !== '') {
    children.push(condition(KEY_COLUMN, 'contains', search.text))
  }
  if (search.type !== '') {
    children.push(condition(TYPE_COLUMN, 'eq', search.type))
  }

  return children.length === 0 ? NO_FILTER : { ...NO_FILTER, children }
}

function condition(column: string, operator: Condition['operator'], value: string): Condition {
  return { kind: 'condition', column, operator, values: [value], enabled: true }
}
