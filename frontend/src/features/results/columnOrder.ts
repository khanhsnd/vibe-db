import type { Namespace } from '@/bridge/connections'

/**
 * The order a reader dragged a grid's columns into.
 *
 * It is a list of **names** rather than of positions: a column dropped from the table, one added to
 * it, or a field a document store's sample happened to miss must not take every column after it
 * along. A name the grid has not got is skipped, and a column the order has never heard of goes
 * last — so an order outlives the table changing under it.
 *
 * **Who keeps one is the call site**, exactly as who sorts is: a table outlives the app, so its
 * order is stored; a query result's lives as long as its statement does and is not stored at all.
 */

/** The order of a grid nobody has arranged, and a referentially stable default for one. */
export const NO_ORDER: string[] = []

/** One column in the reader's order, with the place it holds in a row — which never moves. */
export interface Placed<T> {
  column: T
  index: number
}

/**
 * The columns in the reader's order, each carrying the place it holds in a row. Everything that
 * reads a cell reads that place, so arranging the header never touches the data under it.
 */
export function arrange<T extends { name: string }>(
  columns: T[],
  order: readonly string[],
): Placed<T>[] {
  if (order.length === 0) return columns.map((column, index) => ({ column, index }))

  const places = new Map(columns.map((column, index) => [column.name, { column, index }]))
  const named = new Set(order)

  return [
    ...order.flatMap((name) => {
      const one = places.get(name)
      return one === undefined ? [] : [one]
    }),
    ...columns.flatMap((column, index) => (named.has(column.name) ? [] : [{ column, index }])),
  ]
}

/**
 * The names with `name` taken out and put back **before** `before` — or last, where the reader
 * dropped it past every column there is.
 */
export function moveName(names: string[], name: string, before: string | undefined): string[] {
  const rest = names.filter((one) => one !== name)
  const at = before === undefined ? -1 : rest.indexOf(before)

  return at < 0 ? [...rest, name] : [...rest.slice(0, at), name, ...rest.slice(at)]
}

/** What names a table's own order: which connection, which namespace, which table. */
export const columnOrderKey = (profileId: string, namespace: Namespace, table: string) =>
  `${profileId}:${namespace.database}.${namespace.schema}.${table}`

const STORE_KEY = 'vibe-db.columns'

/** Every stored order in one entry: a reader has as many of them as they have tables. */
function stored(): Record<string, string[]> {
  const raw = localStorage.getItem(STORE_KEY)
  if (raw === null) return {}

  try {
    return JSON.parse(raw) as Record<string, string[]>
  } catch {
    return {}
  }
}

export function loadColumnOrder(key: string): string[] {
  return stored()[key] ?? NO_ORDER
}

export function saveColumnOrder(key: string, order: string[]): void {
  localStorage.setItem(STORE_KEY, JSON.stringify({ ...stored(), [key]: order }))
}
