import { Loader2 } from 'lucide-react'

import { isCancel, ResultPane } from '@/features/results/ResultPane'
import { ResultTabs } from '@/features/results/ResultTabs'
import { cn } from '@/lib/cn'
import type { QueryRun } from '@/stores/queries'

const numbers = new Intl.NumberFormat('en-US')

interface QueryOutputProps {
  run: QueryRun
  sessionId: string | null
  onRefresh: () => void
  onStop: () => void
}

export function QueryOutput({ run, sessionId, onRefresh, onStop }: QueryOutputProps) {
  const panes = [
    {
      id: 'result',
      label: 'Result',
      content: (
        <ResultPane run={run} sessionId={sessionId} onRefresh={onRefresh} onStop={onStop} />
      ),
    },
  ]

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div className="min-h-0 flex-1">
        <ResultTabs panes={panes} />
      </div>

      <StatusStrip run={run} />
    </div>
  )
}

function StatusStrip({ run }: { run: QueryRun }) {
  const { running, result, error } = run

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
          <span className="shrink-0 font-mono">
            {result.columns.length > 0
              ? `${numbers.format(result.rows.length)} rows`
              : `${numbers.format(result.rowsAffected)} affected`}
          </span>
          <span className="text-faint">·</span>
          <span className="shrink-0 font-mono">{numbers.format(result.elapsedMs)} ms</span>
          {result.truncated ? (
            <>
              <span className="text-faint">·</span>
              <span className="shrink-0 text-warn">
                capped at the row limit, more rows on the server
              </span>
            </>
          ) : null}
        </>
      ) : (
        <span className="shrink-0 text-faint">Idle</span>
      )}
    </div>
  )
}
