import { useMemo } from 'react'

import type { QueryPlan } from '@/bridge/query'

import { DocumentView } from './DocumentView'
import { ResultGrid } from './ResultGrid'
import { ResultTabs, type ResultPaneDef } from './ResultTabs'
import { planRows, planStatistics } from './plan'

/**
 * A plan in the three ways one is read, and they are three because they answer different questions:
 * the **grid** is the engine's own EXPLAIN, the answer a reader who knows their engine came for; the
 * **JSON** is the plan document, which is where a step's cost and the shape of the tree actually
 * live; and the **statistics** say where the cost goes, which is the question somebody explaining a
 * slow statement arrived with.
 *
 * Two of the three are `ResultGrid`s, that being the one component for rows and columns.
 */
export function ExplainPane({ plan }: { plan: QueryPlan }) {
  const explained = useMemo(() => planRows(plan), [plan])
  const statistics = useMemo(() => planStatistics(plan), [plan])

  const panes: ResultPaneDef[] = [
    { id: 'explained', label: 'Grid', content: <ResultGrid result={explained} /> },
    {
      id: 'json',
      label: 'JSON',
      content: (
        <div className="flex h-full min-h-0 flex-col">
          <DocumentView text={plan.json} notation="json" />
        </div>
      ),
    },
    { id: 'statistics', label: 'Statistics', content: <ResultGrid result={statistics} /> },
  ]

  return <ResultTabs panes={panes} />
}
