import { useEffect, useMemo, useState } from 'react'
import { AlertTriangle, CheckCircle2, CircleSlash, Terminal } from 'lucide-react'

import type { AppError } from '@/bridge/errors'
import { rowExport, type ExportFormat } from '@/bridge/export'
import { queries, type QueryResult } from '@/bridge/query'
import { ALL_ROWS } from '@/bridge/rows'
import type { TableColumn } from '@/bridge/tables'
import { cn } from '@/lib/cn'
import { CopyButton } from '@/shared/ui/CopyButton'
import { Empty } from '@/shared/ui/Empty'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'
import type { QueryRun } from '@/stores/queries'

import { CellEditor, EDITOR_HEIGHT, EDITOR_MIN_HEIGHT } from './CellEditor'
import { arrange, moveName, NO_ORDER } from './columnOrder'
import { GridFooter } from './GridFooter'
import { GridPager } from './GridPager'
import { GridStatus } from './GridStatus'
import { ResultGrid } from './ResultGrid'
import {
  focusOf,
  NO_SORT,
  oneRange,
  rowRange,
  selectedRows,
  type CellRef,
  type Selection,
  type SortKey,
} from './resultSet'
import { useGridEdits, type EditSource } from './useGridEdits'
import { useGridOrder } from './useGridOrder'
import { usePageWindow } from './usePageWindow'

const numbers = new Intl.NumberFormat('en-US')

/**
 * The columns of no result at all, held steady so the order below is not rebuilt per render. It is
 * a result's own column type rather than the grid's, so standing in for one leaves the type alone —
 * a union of the two would settle `arrange`'s on whichever came first.
 */
const NO_COLUMNS: TableColumn[] = []

/** Stopping a query is a choice, not a failure, so it is reported without alarm. */
const isCancel = (error: AppError) => error.code === 'QUERY_CANCELLED'

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
  const [sort, setSort] = useState<readonly SortKey[]>(NO_SORT)
  const [columnOrder, setColumnOrder] = useState<string[]>(NO_ORDER)
  const [cellShown, setCellShown] = useState(false)
  const { running, result, error } = run

  const grid = result !== null && result.columns.length > 0 ? result : null
  const editable = grid !== null && grid.table !== '' && sessionId !== null
  const edits = useGridEdits(sourceOf(grid), sessionId, onRefresh)

  /** The columns in the reader's own order, each keeping the place it holds in a row. */
  const arranged = useMemo(
    () => arrange(grid?.columns ?? NO_COLUMNS, columnOrder),
    [grid, columnOrder],
  )

  /**
   * The rows are already in memory, so the sort is too — there is no statement to re-order. Only
   * the rows *fetched* are ordered: a draft row stays after them, so Add still lands on the last
   * page.
   */
  const order = useGridOrder(edits.rows, grid?.columns ?? NO_COLUMNS, sort, grid?.rows.length ?? 0)
  const ordered = useMemo(() => order.map((row) => edits.rows[row] ?? []), [order, edits.rows])
  const paged = usePageWindow(ordered)

  /** What the grid draws: this page's rows, under the columns in the order they were arranged. */
  const shown = useMemo(
    () => ({
      columns: arranged.map((one) => one.column),
      rows: paged.rows.map((row) => arranged.map((one) => row[one.index] ?? null)),
    }),
    [arranged, paged.rows],
  )

  /**
   * Lays a value out for the value viewer — the engine's own `Format`, since a document store's
   * documents are its own notation and not JSON. Held steady across renders because the viewer
   * calls it from an effect; a result read with no session left cannot be laid out at all.
   */
  const formatValue = useMemo(
    () => (sessionId === null ? undefined : (value: string) => queries.format(sessionId, value)),
    [sessionId],
  )

  // A selection names rows of the result that produced it, so it never survives a new one.
  useEffect(() => setSelection(null), [grid])

  // The page number and the sort survive a read of the same statement — growing the fetch, or
  // reloading after an edit, must leave the reader where they were — but not a different one,
  // whose columns the sort may not even name.
  const statement = run.request?.sql ?? ''
  useEffect(() => {
    setSort(NO_SORT)
    setColumnOrder(NO_ORDER)
    paged.goToPage(0)
  }, [statement, paged.goToPage])

  /**
   * The grid counts rows within the page, and in the order it drew them; the edits hook counts
   * them across the whole result, in the order they were read.
   */
  const absolute = (row: number) => order[row + paged.offset] ?? row + paged.offset

  /** Columns are counted the same way: the grid draws them arranged, a write names the result's. */
  const realColumn = (column: number) => arranged[column]?.index ?? -1

  /** The cell the editor panel is on — the focus, in the result's own coordinates. */
  const focus = focusOf(selection)
  const entry = focus ? arranged[focus.column] : undefined
  const focused = focus ? paged.rows[focus.row] : undefined
  const target =
    focus && entry && focused
      ? {
          ref: { row: absolute(focus.row), column: entry.index },
          column: entry.column,
          value: focused[entry.index] ?? null,
        }
      : null

  /** The panel edits the focused cell, so the menu's own cell becomes the selection first. */
  const editInPanel = (ref: CellRef) => {
    setSelection(oneRange({ anchor: ref, focus: ref }))
    setCellShown(true)
  }

  /**
   * A query result's column order lives as long as its statement does — there is no stored table
   * under it to remember one against — so it is held here and dropped with the sort.
   */
  const moveColumn = (from: number, to: number) => {
    const name = arranged[from]?.column.name
    if (name === undefined) return

    setSelection(null)
    setColumnOrder(
      moveName(
        arranged.map((one) => one.column.name),
        name,
        arranged[to]?.column.name,
      ),
    )
  }

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
   * A new order only reaches the rows already fetched, so it sends the reader back to the
   * first page: the row that was on this one is now somewhere else entirely.
   */
  const changeSort = (next: SortKey[]) => {
    setSelection(null)
    setSort(next)
    paged.goToPage(0)
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

  /** Rows arrive in the grid's own numbers, which is what `absolute` is for. */
  const deleteRows = (rows: number[]) => edits.deleteRows(rows.map(absolute))

  /**
   * Every row the grid holds, in the order it draws them — the whole result rather than the page
   * in front, since a reader who paged means the result, and the columns arranged as they are on
   * screen. What is on screen is what is written, staged edits included, because the grid is the
   * thing being exported.
   */
  const exportRows = (format: ExportFormat) => {
    if (grid === null || sessionId === null) return

    void rowExport.run(sessionId, format, {
      namespace: { database: grid.database, schema: grid.schema },
      table: grid.table,
      columns: arranged.map((one) => one.column.name),
      rows: ordered.map((row) => arranged.map((one) => row[one.index] ?? null)),
    })
  }

  /**
   * Which panels are open, named for the splitter — see `TableDataView` on `panelIds`. The editor
   * goes with the grid: its only way in and out is the button on that grid's head strip, and a
   * statement that answered with an error rather than rows would otherwise leave it stranded open.
   */
  const cellOpen = cellShown && grid !== null
  const rowPanels = useMemo(() => (cellOpen ? ['rows', 'cell'] : ['rows']), [cellOpen])

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <Split
        id="vibe-db.result.rows"
        orientation="vertical"
        panelIds={rowPanels}
        className="min-h-0 flex-1"
      >
        <SplitPanel id="rows" minSize="25%">
          {error !== null ? (
            <ErrorPanel error={error} />
          ) : grid !== null ? (
            <ResultGrid
              result={shown}
              selection={selection}
              onSelectionChange={setSelection}
              onEditCell={
                editable
                  ? (ref, value) =>
                      edits.setCell(
                        { row: absolute(ref.row), column: realColumn(ref.column) },
                        value,
                      )
                  : undefined
              }
              isDirty={(row, column) => edits.isDirty(absolute(row), realColumn(column))}
              isDeleted={(row) => edits.deleted.has(absolute(row))}
              sort={sort}
              onSortChange={changeSort}
              onMoveColumn={moveColumn}
              onExport={sessionId !== null ? exportRows : undefined}
              cellPanel={{ shown: cellShown, onToggle: () => setCellShown(!cellShown) }}
              onEditInPanel={editInPanel}
              onDeleteRows={editable ? deleteRows : undefined}
              onRefresh={onRefresh}
              formatValue={formatValue}
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
        </SplitPanel>

        {cellOpen ? (
          <>
            <SplitHandle orientation="vertical" />

            <SplitPanel
              id="cell"
              defaultSize={EDITOR_HEIGHT}
              minSize={EDITOR_MIN_HEIGHT}
              maxSize="70%"
            >
              {target === null ? (
                <Empty title="No cell selected" hint="Click a cell in the grid to edit its value." />
              ) : (
                <CellEditor
                  key={`${target.ref.row}:${target.ref.column}`}
                  column={target.column}
                  value={target.value}
                  readOnly={
                    editable
                      ? undefined
                      : 'These rows are not one stored table’s, so a value cannot be written back.'
                  }
                  busy={edits.saving}
                  onApply={(value) => edits.setCell(target.ref, value)}
                  formatValue={formatValue}
                />
              )}
            </SplitPanel>
          </>
        ) : null}
      </Split>

      {edits.error !== null ? <ErrorStrip error={edits.error} /> : null}

      <GridFooter
        editable={editable}
        selected={selection !== null}
        pending={edits.pending}
        busy={edits.saving || running}
        onAddRow={addRow}
        onDeleteRow={() => deleteRows(selectedRows(selection))}
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

      {grid !== null ? (
        <GridStatus
          statement={statement}
          // The rows are paged and sorted in memory, so where the reader is means where they are
          // in the grid — the page's offset plus the focused row, not the order they were read in.
          record={paged.offset + (focus?.row ?? 0) + 1}
          total={paged.total}
          partial={grid.truncated}
          page={paged.page + 1}
          pageCount={paged.pageCount}
          loadedAt={run.loadedAt}
        />
      ) : null}
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

/**
 * A failure where the rows would have been. Everything it says is **selectable, and copyable in one
 * click**: what a reader does with a server's rejection is take it somewhere else — a colleague, a
 * search, an issue — and dragging across three lines in a pane that may be forty pixels tall is not
 * how anybody wants to do that. Copy takes the code, the wording and the engine's own detail
 * together, since the three are one answer.
 */
function ErrorPanel({ error }: { error: AppError }) {
  const cancelled = isCancel(error)
  const whole = [error.code, error.message, error.detail].filter(Boolean).join('\n')

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

        <div className="selectable flex min-w-0 flex-1 flex-col gap-1.5">
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

        <CopyButton text={whole} label="Copy the error" />
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
