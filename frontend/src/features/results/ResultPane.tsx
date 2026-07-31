import { useEffect, useState } from 'react'
import { AlertTriangle, CheckCircle2, CircleSlash, Table2, Terminal } from 'lucide-react'

import type { AppError } from '@/bridge/errors'
import type { QueryResult } from '@/bridge/query'
import { cn } from '@/lib/cn'
import { Empty } from '@/shared/ui/Empty'
import type { QueryRun } from '@/stores/queries'

import { GridFooter } from './GridFooter'
import { ResultGrid } from './ResultGrid'
import type { CellRange } from './resultSet'
import { useGridEdits, type EditSource } from './useGridEdits'

const numbers = new Intl.NumberFormat('en-US')

/** Stopping a query is a choice, not a failure, so it is reported without alarm. */
export const isCancel = (error: AppError) => error.code === 'QUERY_CANCELLED'

interface ResultPaneProps {
  run: QueryRun
  sessionId: string | null
  onRefresh: () => void
  onStop: () => void
}

export function ResultPane({ run, sessionId, onRefresh, onStop }: ResultPaneProps) {
  const [selection, setSelection] = useState<CellRange | null>(null)
  const { running, result, error } = run

  const grid = result !== null && result.columns.length > 0 ? result : null
  const editable = grid !== null && grid.table !== '' && sessionId !== null
  const edits = useGridEdits(sourceOf(grid), sessionId, onRefresh)

  // A selection names a row of the result that produced it, so it does not survive a new one.
  useEffect(() => setSelection(null), [grid])

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div className="min-h-0 flex-1">
        {error !== null ? (
          <ErrorPanel error={error} />
        ) : grid !== null ? (
          <ResultGrid
            result={{ columns: grid.columns, rows: edits.rows }}
            selection={selection}
            onSelectionChange={setSelection}
            onEditCell={editable ? edits.setCell : undefined}
            isDirty={edits.isDirty}
            deleted={edits.deleted}
          />
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

      {edits.error !== null ? (
        <p className="shrink-0 truncate border-t border-danger/40 bg-danger-subtle px-2 py-1 text-xs text-danger">
          {edits.error.message}
        </p>
      ) : null}

      <GridFooter
        editable={editable}
        selected={selection !== null}
        pending={edits.pending}
        busy={edits.saving || running}
        onAddRow={edits.addRow}
        onDeleteRow={() => edits.deleteRows(selection)}
        onApply={() => void edits.apply()}
        onDiscard={edits.discard}
        onRefresh={onRefresh}
        onStop={onStop}
        running={running}
      >
        <span className="flex min-w-0 items-center gap-1.5">
          <Table2 size={12} className="shrink-0 text-faint" />
          <span className="truncate font-mono">{label(grid, editable)}</span>
        </span>
      </GridFooter>
    </div>
  )
}

/**
 * Every result's rows go through the hook, read-only ones included, because the grid shows
 * what the hook returns. `table` is empty for a result the backend could not trace, and it is
 * `editable` — not this — that keeps a write off it.
 */
function sourceOf(grid: QueryResult | null): EditSource | null {
  if (grid === null) return null

  return {
    target: { database: grid.database, schema: grid.schema, table: grid.table },
    columns: grid.columns,
    rows: grid.rows,
  }
}

function label(grid: QueryResult | null, editable: boolean): string {
  if (grid === null) return 'No rows'
  if (!editable) return 'Read-only — rows cannot be traced to a single table'

  const qualified = [grid.database, grid.schema, grid.table].filter((part) => part !== '').join('.')
  return `${qualified} — double-click a cell to edit`
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
