import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'

import { failedStatement } from '@/bridge/query'
import { isCancel, ResultPane } from '@/features/results/ResultPane'
import { ResultTabs, type ResultPaneDef } from '@/features/results/ResultTabs'
import { cn } from '@/lib/cn'
import type { QueryRun } from '@/stores/queries'

import { ScriptMessages } from './ScriptMessages'

const numbers = new Intl.NumberFormat('en-US')

interface QueryOutputProps {
  run: QueryRun
  sessionId: string | null
  onRefresh: () => void
  onFetch: (maxRows: number) => void
  onStop: () => void
}

export function QueryOutput({ run, sessionId, onRefresh, onFetch, onStop }: QueryOutputProps) {
  const statements = run.result?.statements ?? []
  // One statement says everything it has to say in the Result pane, so the strip stays a strip
  // until there is a script to account for.
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
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div className="min-h-0 flex-1">
        <ResultTabs panes={panes} selectedId={script ? pane : 'result'} onSelect={setPane} />
      </div>

      <StatusStrip run={run} />
    </div>
  )
}

function StatusStrip({ run }: { run: QueryRun }) {
  const { running, result, error } = run
  const failed = failedStatement(result)
  const statements = result?.statements.length ?? 0

  return (
    <div className="flex h-8 shrink-0 items-center gap-2 border-t border-line bg-panel px-2 text-xs text-muted">
      {running ? (
        <>
          <Loader2 size={12} className="shrink-0 animate-spin text-accent" />
          <span className="shrink-0">Running…</span>
        </>
      ) : error !== null ? (
        <span className={cn('shrink-0', isCancel(error) ? 'text-muted' : 'text-danger')}>
          {isCancel(error) ? 'Cancelled' : 'Failed'}
        </span>
      ) : result !== null ? (
        <>
          {/* A script says how far it got before it says what the last statement returned. */}
          {statements > 1 ? (
            <>
              <span className={cn('shrink-0', failed && 'text-danger')}>
                {failed
                  ? `Stopped at statement ${result.statements.indexOf(failed) + 1} of ${statements}`
                  : `${statements} statements`}
              </span>
              <span className="text-faint">·</span>
            </>
          ) : null}

          <span className="shrink-0 font-mono">
            {result.columns.length > 0
              ? `${numbers.format(result.rows.length)} rows`
              : `${numbers.format(result.rowsAffected)} affected`}
          </span>
          <span className="text-faint">·</span>
          <span className="shrink-0 font-mono">{numbers.format(result.elapsedMs)} ms</span>
        </>
      ) : (
        <span className="shrink-0 text-faint">Idle</span>
      )}
    </div>
  )
}
