import { useEffect, useState } from 'react'

import type { HistoryEntry } from '@/bridge/history'
import { failedStatement } from '@/bridge/query'
import { ResultPane } from '@/features/results/ResultPane'
import { ResultTabs, type ResultPaneDef } from '@/features/results/ResultTabs'
import type { QueryRun } from '@/stores/queries'

import { QueryHistory } from './QueryHistory'
import { ScriptMessages } from './ScriptMessages'

interface QueryOutputProps {
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
 */
export function QueryOutput({
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

  const [pane, setPane] = useState('result')

  // An Execute brings its own outcome forward, wherever the reader had got to — and a script that
  // stopped shows where it stopped, since the Result pane holds whatever ran before that and
  // cannot say what went wrong.
  const { result, error } = run
  useEffect(() => {
    if (result === null && error === null) return
    setPane(failed ? 'messages' : 'result')
  }, [result, error, failed])

  const panes: ResultPaneDef[] = [
    {
      id: 'result',
      label: 'Result',
      content: (
        <ResultPane
          run={run}
          sessionId={sessionId}
          onRefresh={onRefresh}
          onFetch={onFetch}
          onStop={onStop}
        />
      ),
    },
  ]

  // History keeps a fixed place beside Result, because Messages comes and goes with the script that
  // needs it and a pane the reader clicks repeatedly should not move under them.
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
