import { useEffect, useMemo, useState } from 'react'

import type { HistoryEntry } from '@/bridge/history'
import { failedStatement } from '@/bridge/query'
import { ExplainPane } from '@/features/results/ExplainPane'
import { planSummary } from '@/features/results/plan'
import { ResultGrid } from '@/features/results/ResultGrid'
import { ResultPane } from '@/features/results/ResultPane'
import { ResultTabs, type ResultPaneDef } from '@/features/results/ResultTabs'
import type { QueryRun } from '@/stores/queries'

import { QueryHistory } from './QueryHistory'
import { ScriptMessages } from './ScriptMessages'

interface QueryOutputProps {
  tabId: string
  run: QueryRun
  sessionId: string | null
  profileId: string
  onRefresh: () => void
  onFetch: (maxRows: number) => void
  onStop: () => void
  onPickHistory: (entry: HistoryEntry) => void
}

/**
 * The output half of a query tab: the `Result` pane, `History` beside it, and `Messages` as well
 * once there is a script to account for. What the run *did* — its rows, its time, how far it got —
 * is said by the pane itself, on the status line under its grid, rather than by a strip out here
 * that would only repeat the count already under the reader's eyes.
 *
 * **Explain brings two more**, `Summary` and `Explain`, and they sit next to the `Result` they are
 * about rather than at the end of the strip. `Result` is the engine's own EXPLAIN, unchanged and
 * untranslated — MySQL's grid of access types, PostgreSQL's plan text, MongoDB's one document — and
 * the two beside it are that same answer read as a plan.
 */
export function QueryOutput({
  tabId,
  run,
  sessionId,
  profileId,
  onRefresh,
  onFetch,
  onStop,
  onPickHistory,
}: QueryOutputProps) {
  const statements = run.result?.statements ?? []
  const script = statements.length > 1
  const failed = failedStatement(run.result)
  const plan = run.result?.plan ?? null

  // A plan is two panes: what it says about itself, and the plan itself in the three ways one is
  // read. They are built once per plan, the grid drawing whatever `GridData` it is handed.
  const planPanes = useMemo<ResultPaneDef[]>(
    () =>
      plan === null
        ? []
        : [
            { id: 'summary', label: 'Summary', content: <ResultGrid result={planSummary(plan)} /> },
            { id: 'explain', label: 'Explain', content: <ExplainPane plan={plan} /> },
          ],
    [plan],
  )

  const [pane, setPane] = useState('result')

  // An Execute brings its own outcome forward, wherever the reader had got to — and a script that
  // stopped shows where it stopped, since the Result pane holds whatever ran before that and
  // cannot say what went wrong. A run that came back with a plan was an Explain, and the plan is
  // what was asked for.
  const { result, error } = run
  useEffect(() => {
    if (result === null && error === null) return
    if (failed) setPane('messages')
    else setPane(plan === null ? 'result' : 'explain')
  }, [result, error, failed, plan])

  const panes: ResultPaneDef[] = [
    {
      id: 'result',
      label: 'Result',
      content: (
        <ResultPane
          tabId={tabId}
          run={run}
          sessionId={sessionId}
          onRefresh={onRefresh}
          onFetch={onFetch}
          onStop={onStop}
        />
      ),
    },
  ]

  // A plan is about the result beside it, so it goes there rather than at the end of the strip.
  panes.push(...planPanes)

  // The strip runs from what this run produced to what outlives it: the plan panes belong to the
  // result beside them, History to the connection, and Messages to the script that needed it.
  panes.push({
    id: 'history',
    label: 'History',
    content: <QueryHistory profileId={profileId} onPick={onPickHistory} />,
  })

  if (script) {
    panes.push({
      id: 'messages',
      label: `Messages (${statements.length})`,
      content: <ScriptMessages statements={statements} />,
    })
  }

  // Messages is there only for a script, so a single statement run while it was in front puts the
  // reader back on Result rather than on a pane that has gone.
  const shown = panes.some((one) => one.id === pane) ? pane : 'result'

  return (
    <div className="h-full min-h-0 bg-canvas">
      <ResultTabs panes={panes} selectedId={shown} onSelect={setPane} />
    </div>
  )
}
