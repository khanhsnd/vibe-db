import { AlertTriangle, CheckCircle2, CircleSlash, Loader2, Terminal } from 'lucide-react'

import type { AppError } from '@/bridge/errors'
import { ResultGrid } from '@/features/results/ResultGrid'
import { cn } from '@/lib/cn'
import { Empty } from '@/shared/ui/Empty'
import type { QueryRun } from '@/stores/queries'

const numbers = new Intl.NumberFormat('en-US')

/** Stopping a query is a choice, not a failure, so it is reported without alarm. */
const isCancel = (error: AppError) => error.code === 'QUERY_CANCELLED'

export function QueryOutput({ run }: { run: QueryRun }) {
  const { running, result, error } = run
  const hasRows = result !== null && result.columns.length > 0

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div className="min-h-0 flex-1">
        {error !== null ? (
          <ErrorPanel error={error} />
        ) : hasRows ? (
          <ResultGrid result={result} />
        ) : result !== null ? (
          <SuccessPanel rowsAffected={result.rowsAffected} />
        ) : (
          <Empty
            icon={<Terminal size={24} strokeWidth={1.5} />}
            title={running ? 'Running…' : 'No results yet'}
            hint="Execute a statement to see its rows here. Errors appear in this panel too."
          />
        )}
      </div>

      <StatusStrip run={run} />
    </div>
  )
}

function ErrorPanel({ error }: { error: AppError }) {
  const cancelled = isCancel(error)

  return (
    <div className="scroll-thin h-full overflow-auto p-3">
      <div
        className={cn(
          'flex gap-2.5 rounded-md border p-3',
          cancelled ? 'border-line bg-panel' : 'border-danger/40 bg-danger-subtle',
        )}
      >
        {cancelled ? (
          <CircleSlash size={15} className="mt-0.5 shrink-0 text-muted" />
        ) : (
          <AlertTriangle size={15} className="mt-0.5 shrink-0 text-danger" />
        )}

        <div className="flex min-w-0 flex-col gap-1.5">
          <span
            className={cn(
              'font-mono text-2xs tracking-wide uppercase',
              cancelled ? 'text-muted' : 'text-danger',
            )}
          >
            {error.code}
          </span>
          <p className="text-sm break-words text-fg">{error.message}</p>
          {error.detail ? (
            <p className="font-mono text-xs break-words whitespace-pre-wrap text-muted">
              {error.detail}
            </p>
          ) : null}
        </div>
      </div>
    </div>
  )
}

function SuccessPanel({ rowsAffected }: { rowsAffected: number }) {
  return (
    <div className="p-3">
      <div className="flex items-center gap-2.5 rounded-md border border-line bg-panel p-3">
        <CheckCircle2 size={15} className="shrink-0 text-success" />
        <p className="text-sm text-fg">
          Statement finished. {numbers.format(rowsAffected)}{' '}
          {rowsAffected === 1 ? 'row' : 'rows'} affected.
        </p>
      </div>
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
