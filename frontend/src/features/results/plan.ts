import type { QueryPlan } from '@/bridge/query'

import type { Cell, GridData } from './resultSet'

const column = (name: string, type = 'text') => ({ name, type, fullType: type })

/**
 * A cost as the engine means it, the unit being its own and only good for comparing one step with
 * another. **Nothing is drawn where the engine priced nothing** — MySQL prices reading a table and
 * not the sort above it — because a `0.00` there would read as free rather than as unpriced.
 */
const cost = (value: number) => (value === 0 ? '' : value.toFixed(2))

/** A share of the plan's cost, and nothing at all where there was no cost to take a share of. */
const share = (part: number, whole: number) =>
  part === 0 || whole === 0 ? '' : ((part / whole) * 100).toFixed(1)

/**
 * The engine's own EXPLAIN, drawn as it answered: MySQL's grid of access types, PostgreSQL's plan
 * text down one column, MongoDB's one document. Nothing is normalised, ordered or renamed on the way
 * — a reader who reaches for Explain knows these words, and a client that rewrote them would be
 * standing between them and their server.
 */
export function planRows(plan: QueryPlan): GridData {
  return { columns: plan.columns, rows: plan.rows }
}

/**
 * What the plan spends, per kind of step: how many there are, what they cost between them, and what
 * share of the whole that is. It counts a step's **own** cost — what it adds beyond what it reads —
 * so the shares add up to the plan rather than to several times it.
 *
 * The order is dearest first, which is the order the reader is looking for: the step to go and do
 * something about is at the top.
 */
export function planStatistics(plan: QueryPlan): GridData {
  const counted = new Map<string, { count: number; cost: number }>()
  for (const node of plan.nodes) {
    const seen = counted.get(node.type) ?? { count: 0, cost: 0 }
    counted.set(node.type, { count: seen.count + 1, cost: seen.cost + node.selfCost })
  }

  const ranked = [...counted].sort((a, b) => b[1].cost - a[1].cost || b[1].count - a[1].count)
  const whole = ranked.reduce((total, [, one]) => total + one.cost, 0)

  if (!plan.costed) {
    return {
      columns: [column('Node type'), column('Count', 'int')],
      rows: ranked.map(([type, one]): Cell[] => [type, String(one.count)]),
    }
  }

  return {
    columns: [
      column('Node type'),
      column('Count', 'int'),
      column('Cost', 'decimal'),
      column('Cost %', 'decimal'),
    ],
    rows: ranked.map(([type, one]): Cell[] => [
      type,
      String(one.count),
      cost(one.cost),
      share(one.cost, whole),
    ]),
  }
}

/**
 * What the plan says about itself, in the engine's own wording. Which facts there are is the
 * engine's: PostgreSQL prices a plan, MySQL a query block, and a document store says which plan won.
 */
export function planSummary(plan: QueryPlan): GridData {
  return {
    columns: [column('Fact'), column('Value')],
    rows: plan.summary.map((fact): Cell[] => [fact.label, fact.value]),
  }
}
