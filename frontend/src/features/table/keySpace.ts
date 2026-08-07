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
 * How the text in the box is matched against a key's name. It is the **condition's own operator**
 * and nothing more, which is what keeps three ways of searching one mechanism: the driver already
 * answers all three, and each becomes the glob the server itself is narrowed by.
 *
 * `matches` is the reader spelling that glob — `*email*`, `user:?`, `[0-9]` — so it is the one mode
 * whose text is not escaped on the way. The other two are a fragment of a name, `containsfold`
 * ignoring case (which Redis has not got, so it is folded into the pattern: `[eE][mM]…`) and
 * `contains` insisting on it.
 */
export type KeyMatch = 'containsfold' | 'contains' | 'matches'

/** The three, as the menu inside the box offers them and in that order. */
export const KEY_MATCHES: ReadonlyArray<{
  match: KeyMatch
  label: string
  placeholder: string
}> = [
  { match: 'containsfold', label: 'Keys Contain', placeholder: 'Search keys' },
  { match: 'contains', label: 'Keys Contain (Match Case)', placeholder: 'Search keys' },
  { match: 'matches', label: 'Keys Match Pattern', placeholder: 'Pattern, e.g. *email*' },
]

/**
 * The glob, because that is the notation the engine itself is in: the statement under the grid is a
 * `SCAN … MATCH`, `redis-cli` and every library speak it, and a reader who types `*email*` into a
 * box over a key space means the pattern. The two Contains modes are there for the reader who means
 * the characters.
 */
export const DEFAULT_KEY_MATCH: KeyMatch = 'matches'

/**
 * What the reader has narrowed a key space to: a fragment of a key's name, how it is matched, and
 * one of the six types.
 *
 * A key space is searched rather than filtered, and this is the whole of what the bar over it holds.
 * It is **the same conditions the builder would have written** — `Key contains …` and `Type = …` —
 * because the driver already answers exactly those: it hands the first to the server as the one
 * narrowing Redis has (`SCAN … MATCH *text*`) and applies the second to what comes back. Spelling
 * them as two boxes rather than as a tree is a shorter way of saying the same thing, not a second
 * mechanism, which is why the panel can simply be left out.
 *
 * The **match is the tab's** rather than the conditions', because an empty box writes no condition
 * at all and a mode picked before a word is typed would go with it.
 */
export interface KeySearch {
  text: string
  match: KeyMatch
  type: string
}

export const NO_SEARCH: KeySearch = { text: '', match: DEFAULT_KEY_MATCH, type: '' }

/** What the boxes show: the conditions in force, and the match the tab is holding. */
export function readSearch(criteria: Group, match: KeyMatch): KeySearch {
  const search = { text: '', match, type: '' }

  for (const child of criteria.children) {
    if (child.kind !== 'condition') continue
    // Whichever of the three operators wrote it: the box holds the reader's text, and the menu
    // beside it is what says how that text is being read.
    if (child.column === KEY_COLUMN) search.text = child.values[0] ?? ''
    if (child.column === TYPE_COLUMN && child.operator === 'eq') search.type = child.values[0] ?? ''
  }

  return search
}

/**
 * The two boxes as the tree the read is narrowed by. An empty box says nothing rather than saying
 * `contains ''`, which every key would satisfy — and a search of neither is `NO_FILTER`, the tree a
 * table nobody has narrowed carries.
 */
export function writeSearch(search: KeySearch): Group {
  const children: Condition[] = []

  if (search.text !== '') {
    children.push(condition(KEY_COLUMN, search.match, search.text))
  }
  if (search.type !== '') {
    children.push(condition(TYPE_COLUMN, 'eq', search.type))
  }

  return children.length === 0 ? NO_FILTER : { ...NO_FILTER, children }
}

function condition(column: string, operator: Condition['operator'], value: string): Condition {
  return { kind: 'condition', column, operator, values: [value], enabled: true }
}
