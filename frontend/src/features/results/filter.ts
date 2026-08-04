import type { Cell } from '@/bridge/tables'

import { isNumeric } from './columnType'
import { cellText, type ResultColumn } from './resultSet'

/**
 * The comparisons a filter may make, named by the token the server reads rather than by the
 * operator itself — see `driver.TableFilterOp`, which spells it — so nothing a reader typed
 * arrives as SQL.
 */
export type FilterOp = 'eq' | 'ne' | 'gt' | 'ge' | 'lt' | 'le' | 'contains' | 'isnull' | 'notnull'

/** One condition on one column. `value` is unused by the two operators that take none. */
export interface Filter {
  column: string
  operator: FilterOp
  value: string
}

/**
 * How each comparison reads, and whether it takes a value. The order is the order the cell
 * menu offers them in, which is the order they are usually reached for.
 */
export const FILTER_OPS: ReadonlyArray<{ operator: FilterOp; symbol: string; valueless?: true }> = [
  { operator: 'eq', symbol: '=' },
  { operator: 'ne', symbol: '≠' },
  { operator: 'gt', symbol: '>' },
  { operator: 'ge', symbol: '≥' },
  { operator: 'lt', symbol: '<' },
  { operator: 'le', symbol: '≤' },
  { operator: 'contains', symbol: 'contains' },
  { operator: 'isnull', symbol: 'is NULL', valueless: true },
  { operator: 'notnull', symbol: 'is not NULL', valueless: true },
]

const symbolOf = (operator: FilterOp) =>
  FILTER_OPS.find((one) => one.operator === operator)?.symbol ?? operator

const takesValue = (operator: FilterOp) =>
  FILTER_OPS.find((one) => one.operator === operator)?.valueless !== true

/** How a filter reads on the strip and in the menu: `email contains shop`, `deleted_at is NULL`. */
export function filterLabel(filter: Filter): string {
  const comparison = symbolOf(filter.operator)
  return takesValue(filter.operator)
    ? `${filter.column} ${comparison} ${filter.value}`
    : `${filter.column} ${comparison}`
}

/** What the menu offers over one cell: the comparison, then the value it would compare with. */
export function operatorLabel(operator: FilterOp, value: Cell): string {
  return takesValue(operator) ? `${symbolOf(operator)} ${cellText(value)}` : symbolOf(operator)
}

/**
 * Whether a row satisfies every filter — the in-memory half of filtering, for a query result,
 * whose rows are already here. A table read is filtered by its own `WHERE` instead, and the two
 * agree except where an engine's collation has its own opinion: `contains` ignores case here.
 */
export function matchesFilters(
  row: Cell[],
  columns: ResultColumn[],
  filters: readonly Filter[],
): boolean {
  return filters.every((filter) => {
    const index = columns.findIndex((column) => column.name === filter.column)
    if (index < 0) return true

    return holds(row[index] ?? null, filter, isNumeric(columns[index]?.type ?? ''))
  })
}

function holds(value: Cell, filter: Filter, numeric: boolean): boolean {
  if (filter.operator === 'isnull') return value === null
  if (filter.operator === 'notnull') return value !== null
  if (value === null) return false

  if (filter.operator === 'contains') {
    return value.toLowerCase().includes(filter.value.toLowerCase())
  }

  const order = numeric
    ? Number(value) - Number(filter.value)
    : value.localeCompare(filter.value)

  switch (filter.operator) {
    case 'eq':
      return order === 0
    case 'ne':
      return order !== 0
    case 'gt':
      return order > 0
    case 'ge':
      return order >= 0
    case 'lt':
      return order < 0
    case 'le':
      return order <= 0
  }
}

/**
 * What the grid needs to offer filtering: the conditions in force, whether the strip that lists
 * them is showing, and how to change either. A surface that cannot filter passes none of it, and
 * the grid then leaves those items off its menu — the same rule `onEditCell` follows.
 */
export interface GridFiltering {
  filters: Filter[]
  /** Whether the Filter & Sort strip is drawn. The filters hold whether it is or not. */
  shown: boolean
  onShownChange: (shown: boolean) => void
  onChange: (filters: Filter[]) => void
}
