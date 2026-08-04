import type { Cell } from '@/bridge/tables'

import { isNumeric } from './columnType'
import { cellText, type ResultColumn } from './resultSet'

/**
 * The comparisons a filter may make, named by the token the server reads rather than by the
 * operator itself — see `driver.TableFilterOp`, which spells it — so nothing a reader typed
 * arrives as SQL.
 */
export type FilterOp =
  | 'eq'
  | 'ne'
  | 'lt'
  | 'le'
  | 'gt'
  | 'ge'
  | 'contains'
  | 'notcontains'
  | 'begins'
  | 'notbegins'
  | 'ends'
  | 'notends'
  | 'isnull'
  | 'notnull'
  | 'empty'
  | 'notempty'
  | 'between'
  | 'notbetween'
  | 'inlist'
  | 'notinlist'

/** How many values an operator compares against. `list` is any number above none. */
export type Arity = 0 | 1 | 2 | 'list'

/**
 * How each comparison reads and how many values it takes. The order is the order the builder and
 * the cell menu offer them in, which is the order they are usually reached for.
 */
export const FILTER_OPS: ReadonlyArray<{ operator: FilterOp; symbol: string; arity: Arity }> = [
  { operator: 'eq', symbol: '=', arity: 1 },
  { operator: 'ne', symbol: '≠', arity: 1 },
  { operator: 'lt', symbol: '<', arity: 1 },
  { operator: 'le', symbol: '≤', arity: 1 },
  { operator: 'gt', symbol: '>', arity: 1 },
  { operator: 'ge', symbol: '≥', arity: 1 },
  { operator: 'contains', symbol: 'contains', arity: 1 },
  { operator: 'notcontains', symbol: 'does not contain', arity: 1 },
  { operator: 'begins', symbol: 'begins with', arity: 1 },
  { operator: 'notbegins', symbol: 'does not begin with', arity: 1 },
  { operator: 'ends', symbol: 'ends with', arity: 1 },
  { operator: 'notends', symbol: 'does not end with', arity: 1 },
  { operator: 'isnull', symbol: 'is NULL', arity: 0 },
  { operator: 'notnull', symbol: 'is not NULL', arity: 0 },
  { operator: 'empty', symbol: 'is empty', arity: 0 },
  { operator: 'notempty', symbol: 'is not empty', arity: 0 },
  { operator: 'between', symbol: 'is between', arity: 2 },
  { operator: 'notbetween', symbol: 'is not between', arity: 2 },
  { operator: 'inlist', symbol: 'is in list', arity: 'list' },
  { operator: 'notinlist', symbol: 'is not in list', arity: 'list' },
]

/** How the children of a group are joined. Every group joins its own one way. */
export type Conjunction = 'and' | 'or'

/**
 * One condition on one column. An unset value is **absent** from `values` rather than empty,
 * which is what leaves `= ''` sayable: a reader who means the empty string reaches for `is empty`,
 * and a reader who has not typed anything yet has said nothing.
 *
 * `enabled` is the builder's tick box. An unticked condition stays where it was written and
 * narrows nothing, so a reader can put one aside without losing it.
 */
export interface Condition {
  kind: 'condition'
  column: string
  operator: FilterOp
  values: string[]
  enabled: boolean
}

/**
 * The parentheses: children joined by one conjunction. A tree rather than a list because a reader
 * who means `a AND (b OR c)` cannot say it in a list, and those parentheses are the whole point.
 * The root of every tree is one of these, and an empty root narrows nothing.
 */
export interface Group {
  kind: 'group'
  conjunction: Conjunction
  children: Criterion[]
  enabled: boolean
}

export type Criterion = Condition | Group

/** Where a node sits in the tree: the child index at each level down from the root. */
export type Path = number[]

/** The root of a tree that narrows nothing, and the state a surface that has never filtered holds. */
export const NO_FILTER: Group = { kind: 'group', conjunction: 'and', children: [], enabled: true }

export const arityOf = (operator: FilterOp): Arity =>
  FILTER_OPS.find((one) => one.operator === operator)?.arity ?? 1

export const symbolOf = (operator: FilterOp): string =>
  FILTER_OPS.find((one) => one.operator === operator)?.symbol ?? operator

/** What the value part of a condition shows: the placeholder until it holds what it needs. */
export const VALUE_PLACEHOLDER = '<?>'

/** A condition holds every value its operator compares against, and is therefore worth applying. */
export function isComplete(condition: Condition): boolean {
  const arity = arityOf(condition.operator)
  return arity === 'list' ? condition.values.length > 0 : condition.values.length === arity
}

/** How the values read once they are all there: `18 AND 30` for a range, `(a, b)` for a list. */
export function valueLabel(condition: Condition): string {
  if (!isComplete(condition)) return VALUE_PLACEHOLDER

  switch (arityOf(condition.operator)) {
    case 0:
      return ''
    case 2:
      return `${condition.values[0]} AND ${condition.values[1]}`
    case 'list':
      return `(${condition.values.join(', ')})`
    default:
      return condition.values[0] ?? ''
  }
}

/** How a condition reads in one line: `email contains shop`, `deleted_at is NULL`. */
export function conditionLabel(condition: Condition): string {
  const value = valueLabel(condition)
  const comparison = `${condition.column} ${symbolOf(condition.operator)}`

  return value === '' ? comparison : `${comparison} ${value}`
}

/** What the cell menu offers over one cell: the comparison, then the value it would compare with. */
export function operatorLabel(operator: FilterOp, value: Cell): string {
  const arity = arityOf(operator)
  if (arity === 0) return symbolOf(operator)
  // Two values and a list cannot be read off one cell, so the menu offers the placeholder and
  // the builder is where they are filled in.
  if (arity !== 1) return `${symbolOf(operator)} ${VALUE_PLACEHOLDER}`

  return `${symbolOf(operator)} ${cellText(value)}`
}

/**
 * A fresh condition on the first column, which is what `+` adds. `contains` rather than `=`
 * because a reader adding a row is looking for something rather than naming it exactly.
 */
export function newCondition(column: string): Condition {
  return { kind: 'condition', column, operator: 'contains', values: [], enabled: true }
}

/** A fresh empty group, which is what `()+` adds: the parentheses first, their contents after. */
export function newGroup(): Group {
  return { kind: 'group', conjunction: 'or', children: [], enabled: true }
}

/**
 * The tree with the node at `path` replaced, or dropped where `next` is null. Nothing is mutated,
 * so a store holding the old tree still holds it.
 */
export function replaceAt(root: Group, path: Path, next: Criterion | null): Group {
  const [index, ...rest] = path
  if (index === undefined) return root

  const children = [...root.children]
  const child = children[index]
  if (!child) return root

  if (rest.length === 0) {
    if (next === null) children.splice(index, 1)
    else children[index] = next
  } else if (child.kind === 'group') {
    children[index] = replaceAt(child, rest, next)
  } else {
    return root
  }

  return { ...root, children }
}

/**
 * The tree with a node added **after** the one at `path`, so `+` on a row puts the new condition
 * beside it and inside the same parentheses. An empty path appends to the root, which is what the
 * cell menu's own conditions do.
 */
export function insertAfter(root: Group, path: Path, node: Criterion): Group {
  const [index, ...rest] = path
  if (index === undefined) return { ...root, children: [...root.children, node] }

  const children = [...root.children]
  const child = children[index]

  if (rest.length === 0) {
    children.splice(index + 1, 0, node)
  } else if (child?.kind === 'group') {
    children[index] = insertAfter(child, rest, node)
  } else {
    return root
  }

  return { ...root, children }
}

/**
 * The tree with a node appended **inside** the group at `path`, which is what `+` on a group row
 * means: a group is opened before it is filled, so its own row adds to what is between its
 * parentheses rather than beside them.
 */
export function appendTo(root: Group, path: Path, node: Criterion): Group {
  const [index, ...rest] = path
  if (index === undefined) return { ...root, children: [...root.children, node] }

  const child = root.children[index]
  if (child?.kind !== 'group') return root

  const children = [...root.children]
  children[index] = appendTo(child, rest, node)
  return { ...root, children }
}

/** The values that survive a change of operator: as many as the new one takes, and no more. */
export function fitValues(values: string[], arity: Arity): string[] {
  return arity === 'list' ? values : values.slice(0, arity)
}

/**
 * The tree as it actually narrows rows: an unticked node goes, and so does a condition whose
 * values are not all there — a reader half-way through spelling one has not asked for anything
 * yet. A group left holding nothing goes with them, because empty parentheses narrow nothing and
 * `()` is not a clause.
 *
 * Every side reads the same pruned tree: it is what crosses the wire to become a `WHERE`, and what
 * an in-memory result matches its rows against.
 */
export function activeFilter(root: Group): Group {
  const [kept] = prune(root)
  return kept?.kind === 'group' ? kept : NO_FILTER
}

function prune(node: Criterion): Criterion[] {
  if (!node.enabled) return []
  if (node.kind === 'condition') return isComplete(node) ? [node] : []

  const children = node.children.flatMap(prune)
  return children.length > 0 ? [{ ...node, children }] : []
}

/** How many conditions the tree holds, ticked or not — what the Filter & Sort button counts. */
export function countConditions(node: Criterion): number {
  return node.kind === 'condition' ? 1 : node.children.reduce((sum, one) => sum + countConditions(one), 0)
}

/**
 * Whether a row satisfies the tree — the in-memory half of filtering, for a query result whose
 * rows are already here. A table read is filtered by its own `WHERE` instead, and the two agree
 * except where an engine's collation has its own opinion: the text matches ignore case here.
 *
 * The tree is the **pruned** one, from `activeFilter`: pruning once per read beats pruning once
 * per row, and both sides then narrow by exactly the same conditions.
 */
export function matchesFilter(row: Cell[], columns: ResultColumn[], filter: Group): boolean {
  return holdsCriterion(row, columns, filter)
}

function holdsCriterion(row: Cell[], columns: ResultColumn[], node: Criterion): boolean {
  if (node.kind === 'group') {
    if (node.children.length === 0) return true

    return node.conjunction === 'and'
      ? node.children.every((child) => holdsCriterion(row, columns, child))
      : node.children.some((child) => holdsCriterion(row, columns, child))
  }

  const index = columns.findIndex((column) => column.name === node.column)
  if (index < 0) return true

  return holds(row[index] ?? null, node, isNumeric(columns[index]?.type ?? ''))
}

function holds(value: Cell, condition: Condition, numeric: boolean): boolean {
  if (condition.operator === 'isnull') return value === null
  if (condition.operator === 'notnull') return value !== null
  // Every other comparison is against a value, and a NULL is the absence of one.
  if (value === null) return false

  const [first = '', second = ''] = condition.values
  const text = value.toLowerCase()
  const order = (other: string) =>
    numeric ? Number(value) - Number(other) : value.localeCompare(other)

  switch (condition.operator) {
    case 'empty':
      return value === ''
    case 'notempty':
      return value !== ''
    case 'contains':
      return text.includes(first.toLowerCase())
    case 'notcontains':
      return !text.includes(first.toLowerCase())
    case 'begins':
      return text.startsWith(first.toLowerCase())
    case 'notbegins':
      return !text.startsWith(first.toLowerCase())
    case 'ends':
      return text.endsWith(first.toLowerCase())
    case 'notends':
      return !text.endsWith(first.toLowerCase())
    case 'inlist':
      return condition.values.includes(value)
    case 'notinlist':
      return !condition.values.includes(value)
    case 'between':
      return order(first) >= 0 && order(second) <= 0
    case 'notbetween':
      return order(first) < 0 || order(second) > 0
    case 'eq':
      return order(first) === 0
    case 'ne':
      return order(first) !== 0
    case 'lt':
      return order(first) < 0
    case 'le':
      return order(first) <= 0
    case 'gt':
      return order(first) > 0
    case 'ge':
      return order(first) >= 0
  }
}

/**
 * What the grid needs to offer filtering: the tree in force, whether the builder that spells it is
 * showing, and how to change either. A surface that cannot filter passes none of it, and the grid
 * then leaves those items off its menu — the same rule `onEditCell` follows.
 */
export interface GridFiltering {
  /** The root of the conditions in force. Its children are what narrow the rows. */
  criteria: Group
  /** Whether the Filter & Sort panel is drawn. The conditions hold whether it is or not. */
  shown: boolean
  onShownChange: (shown: boolean) => void
  onChange: (criteria: Group) => void
  /**
   * The values a column actually holds, for the value picker to offer instead of making the
   * reader type one. A surface that cannot ask — a query result the backend could not trace back
   * to a table — leaves it out, and the picker is then the box alone.
   */
  suggest?: ((column: string) => Promise<string[]>) | undefined
}
