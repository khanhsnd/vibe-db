import { useEffect, useState } from 'react'
import { AlertTriangle, CheckCircle2, CircleSlash, Terminal } from 'lucide-react'

import type { AppError } from '@/bridge/errors'
import type { QueryResult } from '@/bridge/query'
import { ALL_ROWS } from '@/bridge/rows'
import { cn } from '@/lib/cn'
import { Empty } from '@/shared/ui/Empty'
import type { QueryRun } from '@/stores/queries'

import { GridFooter } from './GridFooter'
import { GridPager } from './GridPager'
import { ResultGrid } from './ResultGrid'
import { oneRange, rowRange, selectedRows, type Selection } from './resultSet'
import { useGridEdits, type EditSource } from './useGridEdits'
import { usePageWindow } from './usePageWindow'

const numbers = new Intl.NumberFormat('en-US')

/** Stopping a query is a choice, not a failure, so it is reported without alarm. */
export const isCancel = (error: AppError) => error.code === 'QUERY_CANCELLED'

interface ResultPaneProps {
  run: QueryRun
  sessionId: string | null
  onRefresh: () => void
  /** Replays the statement asking for this many rows, so the pager can reach past the first read. */
  onFetch: (maxRows: number) => void
  onStop: () => void
}

export function ResultPane({ run, sessionId, onRefresh, onFetch, onStop }: ResultPaneProps) {
  const [selection, setSelection] = useState<Selection | null>(null)
  const { running, result, error } = run

  const grid = result !== null && result.columns.length > 0 ? result : null
  const editable = grid !== null && grid.table !== '' && sessionId !== null
  const edits = useGridEdits(sourceOf(grid), sessionId, onRefresh)
  const paged = usePageWindow(edits.rows)

  // A selection names rows of the result that produced it, so it never survives a new one.
  useEffect(() => setSelection(null), [grid])

  // The page number survives a read of the same statement — growing the fetch, or reloading
  // after an edit, must leave the reader where they were — but not a different statement.
  const statement = run.request?.sql ?? ''
  useEffect(() => {
    paged.goToPage(0)
  }, [statement, paged.goToPage])

  /** The grid counts rows within the page; the edits hook counts them across the whole result. */
  const absolute = (row: number) => row + paged.offset

  /**
   * The fetch is a prefix of the result and the pager grows it: a page beyond what the last read
   * returned replays the statement asking for enough rows to fill it. Everything already held is
   * still paged in memory, so only reaching past the prefix costs a read.
   */
  const reach = (page: number, size: number) => {
    if (grid === null || !grid.truncated) return

    if (size === ALL_ROWS) onFetch(ALL_ROWS)
    else if ((page + 1) * size > grid.rows.length) onFetch((page + 1) * size)
  }

  const changePage = (next: number) => {
    setSelection(null)
    paged.goToPage(next)
    reach(next, paged.size)
  }

  const changeSize = (next: number) => {
    setSelection(null)
    paged.changeSize(next)
    reach(0, next)
  }

  /**
   * A draft row lands after every fetched row, so the page holding it is the one to show — and
   * it becomes the selection, because a paste that follows the add means that row.
   */
  const addRow = () => {
    const at = paged.lastPlace(edits.rows.length + 1)

    edits.addRow()
    paged.goToPage(at.page)
    setSelection(oneRange(rowRange(at.row, grid?.columns.length ?? 0)))
  }

  const deleteRows = () => edits.deleteRows(selectedRows(selection).map(absolute))

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div className="min-h-0 flex-1">
        {error !== null ? (
          <ErrorPanel error={error} />
        ) : grid !== null ? (
          <ResultGrid
            result={{ columns: grid.columns, rows: paged.rows }}
            selection={selection}
            onSelectionChange={setSelection}
            onEditCell={
              editable
                ? (ref, value) => edits.setCell({ ...ref, row: absolute(ref.row) }, value)
                : undefined
            }
            isDirty={(row, column) => edits.isDirty(absolute(row), column)}
            isDeleted={(row) => edits.deleted.has(absolute(row))}
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
        onAddRow={addRow}
        onDeleteRow={deleteRows}
        onApply={() => void edits.apply()}
        onDiscard={edits.discard}
        onRefresh={onRefresh}
        onStop={onStop}
        running={running}
      >
        {grid !== null ? (
          <GridPager
            page={paged.page}
            size={paged.size}
            total={paged.total}
            partial={grid.truncated}
            loading={running}
            onPageChange={changePage}
            onSizeChange={changeSize}
          />
        ) : null}
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
