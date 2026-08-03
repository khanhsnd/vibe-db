import { useEffect, useState } from 'react'

import { failedStatement } from '@/bridge/query'
import { ResultPane } from '@/features/results/ResultPane'
import { ResultTabs, type ResultPaneDef } from '@/features/results/ResultTabs'
import type { QueryRun } from '@/stores/queries'

import { ScriptMessages } from './ScriptMessages'

interface QueryOutputProps {
  run: QueryRun
  sessionId: string | null
  onRefresh: () => void
  onFetch: (maxRows: number) => void
  onStop: () => void
}

/**
 * The output half of a query tab: the `Result` pane, and a `Messages` pane beside it once there
 * is a script to account for. What the run *did* — its rows, its time, how far it got — is said by
 * the pane itself, on the status line under its grid, rather than by a strip out here that would
 * only repeat the count already under the reader's eyes.
 */
export function QueryOutput({ run, sessionId, onRefresh, onFetch, onStop }: QueryOutputProps) {
  const statements = run.result?.statements ?? []
  const script = statements.length > 1
  const failed = failedStatement(run.result)

  const [pane, setPane] = useState('result')

  // A script that stopped shows where it stopped, since the Result pane holds whatever ran
  // before that and cannot say what went wrong.
  useEffect(() => setPane(failed ? 'messages' : 'result'), [failed])

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

  if (script) {
    panes.push({
      id: 'messages',
      label: `Messages (${statements.length})`,
      content: <ScriptMessages statements={statements} />,
    })
  }

  return (
    <div className="h-full min-h-0 bg-canvas">
      <ResultTabs panes={panes} selectedId={script ? pane : 'result'} onSelect={setPane} />
    </div>
  )
}
