import { activeFilter, type Condition, type Criterion, type FilterOp, type Group } from './filter'
import { isNumeric } from './columnType'
import type { Cell, ResultColumn } from './resultSet'

/**
 * Applies a filter tree to rows that are **already in memory**, and answers with the row numbers that
 * survive it.
 *
 * It is the in-memory counterpart of the `WHERE` a table view sends: there, the same tree is spelled
 * by a driver and the server decides. This exists for the surface whose rows never came from a query
 * the reader could have narrowed — a [server monitor's panes](../../../../docs/features/monitor.md),
 * where a read is one catalogue answer and one of the engines (`SLOWLOG GET`) will not take a
 * condition at all. Filtering there is the only kind there could be, and doing it the same way on all
 * six keeps the panes answering alike.
 *
 * It answers with **indexes** rather than with rows, for the reason `sortedOrder` does: a monitor's
 * rows travel beside references, and a narrowed list whose references were the unnarrowed ones would
 * act on the wrong row.
 *
 * Two ways it deliberately differs from the server, both because there is no one server to agree
 * with — these panes hold six engines' catalogues:
 *
 *   - **The text comparisons fold case.** `contains`, `begins with` and `ends with` ignore it, which
 *     is what the box above the grid already does and what a reader hunting a statement means.
 *   - **A numeric column compares as numbers**, decided by the column's own type exactly as a sort
 *     is, and everything else compares as text. A server's collation is its own and is not
 *     reproduced here.
 */
export function filteredRows(
  rows: Cell[][],
  columns: ResultColumn[],
  criteria: Group,
): number[] {
  const active = activeFilter(criteria)
  const order = rows.map((_, index) => index)
  if (active.children.length === 0) return order

  // Where each column sits, and whether it reads as a number. Read once: a filter over two hundred
  // rows would otherwise search the column list per cell.
  const at = new Map(columns.map((column, index) => [column.name, index]))
  const numeric = new Map(columns.map((column) => [column.name, isNumeric(column.type)]))

  return order.filter((index) => holds(active, rows[index] ?? [], at, numeric))
}

/** Whether one row satisfies a node: a group by its conjunction, a condition by its operator. */
function holds(
  node: Criterion,
  row: Cell[],
  at: Map<string, number>,
  numeric: Map<string, boolean>,
): boolean {
  if (node.kind === 'condition') return matches(node, row, at, numeric)

  // An empty group narrows nothing, whichever conjunction it carries. `activeFilter` has already
  // dropped the ones that were merely unticked or half-written.
  if (node.children.length === 0) return true
  return node.conjunction === 'and'
    ? node.children.every((child) => holds(child, row, at, numeric))
    : node.children.some((child) => holds(child, row, at, numeric))
}

function matches(
  condition: Condition,
  row: Cell[],
  at: Map<string, number>,
  numeric: Map<string, boolean>,
): boolean {
  const index = at.get(condition.column)
  // A condition on a column this pane has not got holds of every row rather than of none. The panes
  // keep different columns, and a condition left over from another one should not empty the grid.
  if (index === undefined) return true

  const value = row[index] ?? null
  return decides(condition.operator, value, condition.values, numeric.get(condition.column) === true)
}

/**
 * The whole of what an operator means over one value. The four that ask nothing of a value come
 * first, because they are the four that read a NULL as an answer rather than as a missing one: every
 * comparison below them is false for a NULL, which is the rule SQL itself has.
 */
function decides(operator: FilterOp, value: Cell, values: string[], asNumber: boolean): boolean {
  switch (operator) {
    case 'isnull':
      return value === null
    case 'notnull':
      return value !== null
    case 'empty':
      return value === ''
    case 'notempty':
      return value !== ''
  }

  if (value === null) return false

  const first = values[0] ?? ''
  const folded = value.toLowerCase()
  const needle = first.toLowerCase()

  switch (operator) {
    case 'eq':
      return compare(value, first, asNumber) === 0
    case 'ne':
      return compare(value, first, asNumber) !== 0
    case 'lt':
      return compare(value, first, asNumber) < 0
    case 'le':
      return compare(value, first, asNumber) <= 0
    case 'gt':
      return compare(value, first, asNumber) > 0
    case 'ge':
      return compare(value, first, asNumber) >= 0

    case 'contains':
    case 'containsfold':
      return folded.includes(needle)
    case 'notcontains':
      return !folded.includes(needle)
    case 'begins':
      return folded.startsWith(needle)
    case 'notbegins':
      return !folded.startsWith(needle)
    case 'ends':
      return folded.endsWith(needle)
    case 'notends':
      return !folded.endsWith(needle)

    case 'between':
      return (
        compare(value, first, asNumber) >= 0 && compare(value, values[1] ?? '', asNumber) <= 0
      )
    case 'notbetween':
      return !(
        compare(value, first, asNumber) >= 0 && compare(value, values[1] ?? '', asNumber) <= 0
      )

    case 'inlist':
      return values.some((one) => compare(value, one, asNumber) === 0)
    case 'notinlist':
      return !values.some((one) => compare(value, one, asNumber) === 0)

    // A pattern in the **engine's own** notation, which this side has no way to honour: the builder
    // never writes one, and a key space — the surface that does — narrows on the server. Reading it
    // as a JavaScript expression would be quietly answering a different question, so it holds of
    // every row and the reader sees an unnarrowed list rather than a wrongly narrowed one.
    case 'matches':
      return true
  }
}

/** Below, above or equal — as numbers where the column is one, and as text everywhere else. */
function compare(value: string, against: string, asNumber: boolean): number {
  if (!asNumber) return value.localeCompare(against)

  const left = Number(value)
  const right = Number(against)
  // A column that reads as numeric can still hold something that is not one — a `TIME(6)` duration,
  // an engine's own word for a missing measurement. Such a pair is compared as the text it is rather
  // than as two NaNs, which would have compared equal.
  if (Number.isNaN(left) || Number.isNaN(right)) return value.localeCompare(against)
  return left - right
}
