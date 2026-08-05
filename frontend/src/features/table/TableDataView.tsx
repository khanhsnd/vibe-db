import { useMemo, useState } from 'react'

import type { Namespace } from '@/bridge/connections'
import { rowExport, type ExportFormat } from '@/bridge/export'
import { queries } from '@/bridge/query'
import { tables, type TablePage } from '@/bridge/tables'
import {
  CellEditor,
  EDITOR_HEIGHT,
  EDITOR_MIN_HEIGHT,
} from '@/features/results/CellEditor'
import { GridFooter } from '@/features/results/GridFooter'
import { GridPager } from '@/features/results/GridPager'
import { GridStatus } from '@/features/results/GridStatus'
import {
  arrange,
  columnOrderKey,
  loadColumnOrder,
  moveName,
  saveColumnOrder,
} from '@/features/results/columnOrder'
import { countConditions, type Group } from '@/features/results/filter'
import { pageCount } from '@/features/results/paging'
import { ResultGrid } from '@/features/results/ResultGrid'
import {
  focusOf,
  oneRange,
  rowRange,
  selectedRows,
  type CellRef,
  type Selection,
  type SortKey,
} from '@/features/results/resultSet'
import { useGridEdits, type EditSource } from '@/features/results/useGridEdits'
import { Empty } from '@/shared/ui/Empty'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { FetchProgress } from '@/shared/ui/FetchProgress'
import { Loading } from '@/shared/ui/Loading'
import { ICON } from '@/shared/ui/menuIcons'
import { Split, SplitHandle, SplitPanel } from '@/shared/ui/Split'
import { IDLE_VIEW, useTableViewsStore } from '@/stores/tableViews'

import { ColumnPicker } from './ColumnPicker'
import { TableToolbar, type PanelKind } from './TableToolbar'
import { useTableData } from './useTableData'

// What the column list opens at, and how far it may be dragged. Only the size it opens at is a look —
// the bounds are what stop a drag leaving a panel too small to read or the grid with nothing left.
// The cell editor's own three are `CellEditor`'s, being the same panel on two surfaces.
const PICKER_WIDTH = 168
const PICKER_MIN_WIDTH = 120
const PICKER_MAX_WIDTH = 420

interface TableDataViewProps {
  sessionId: string
  /** Which connection this table is on — half of what names the column order it remembers. */
  profileId: string
  namespace: Namespace
  table: string
  /** Which tab this is, and so which view is the reader's — see `stores/tableViews.ts`. */
  tabId: string
}

export function TableDataView({
  sessionId,
  profileId,
  namespace,
  table,
  tabId,
}: TableDataViewProps) {
  const {
    data,
    loadedAt,
    loading,
    fetch,
    error,
    page,
    limit,
    sort,
    criteria,
    goToPage,
    changeLimit,
    changeSort,
    changeCriteria,
    refresh,
    stopFetch,
  } = useTableData(sessionId, namespace, table, tabId)

  // A table outlives the app, so the order its columns were dragged into is stored under it. The
  // tab is keyed on its own target, so the key is read once and never changes under the component.
  const orderKey = columnOrderKey(profileId, namespace, table)
  const [order, setOrder] = useState(() => loadColumnOrder(orderKey))

  // How the tab was set up is the tab's and survives it being looked away from; the selection is
  // not — it names rows of a read this mount is about to make again.
  const { panels, hidden, filterShown } = useTableViewsStore((s) => s.views[tabId] ?? IDLE_VIEW)
  const change = useTableViewsStore((s) => s.change)
  const [selection, setSelection] = useState<Selection | null>(null)

  const setFilterShown = (shown: boolean) => change(tabId, { filterShown: shown })

  const columns = data?.columns ?? []
  const editable = columns.some((column) => column.primaryKey)
  const edits = useGridEdits(sourceOf(data, namespace, table), sessionId, refresh)

  /** The columns in the reader's own order, each keeping the place it holds in a row. */
  const arranged = useMemo(() => arrange(columns, order), [columns, order])

  const visible = useMemo(
    () => arranged.filter(({ column }) => !hidden.has(column.name)),
    [arranged, hidden],
  )

  const grid = useMemo(
    () => ({
      columns: visible.map((entry) => entry.column),
      rows: edits.rows.map((row) => visible.map((entry) => row[entry.index] ?? null)),
    }),
    [edits.rows, visible],
  )

  const target = useMemo(() => {
    const focus = focusOf(selection)
    if (!focus) return null

    const entry = visible[focus.column]
    const row = edits.rows[focus.row]
    if (!entry || !row) return null

    return {
      ref: { row: focus.row, column: entry.index },
      column: entry.column,
      value: row[entry.index] ?? null,
    }
  }, [selection, visible, edits.rows])

  /** The grid reports positions among the visible columns, which map back to real indexes. */
  const realColumn = (column: number) => visible[column]?.index ?? -1

  /**
   * Where the reader is in the whole table. The rows are paged on the server, so the grid's first
   * row is this page's first row and the read's own offset is what puts it in the table — and with
   * nothing focused, the page's first row is where the reader is.
   */
  const record = data === null ? 0 : data.offset + (focusOf(selection)?.row ?? 0) + 1

  const togglePanel = (kind: PanelKind) => change(tabId, { panels: toggled(panels, kind) })

  const toggleColumn = (name: string) => {
    setSelection(null)
    change(tabId, { hidden: toggled(hidden, name) })
  }

  /** The draft lands after every fetched row, and is selected so a paste that follows means it. */
  const addRow = () => {
    setSelection(oneRange(rowRange(edits.rows.length, visible.length)))
    edits.addRow()
  }

  const showAllColumns = () => {
    setSelection(null)
    change(tabId, { hidden: new Set() })
  }

  /**
   * The grid reports where a column was dropped among the columns it *drew*, so the name it landed
   * before is what places it among them all — a hidden column keeps the place it had. The selection
   * goes, its cell coordinates being positions among those drawn columns.
   */
  const moveColumn = (from: number, to: number) => {
    const name = visible[from]?.column.name
    if (name === undefined) return

    const next = moveName(
      arranged.map((one) => one.column.name),
      name,
      visible[to]?.column.name,
    )

    setSelection(null)
    setOrder(next)
    saveColumnOrder(orderKey, next)
  }

  const changePage = (next: number) => {
    setSelection(null)
    goToPage(next)
  }

  const changePageSize = (next: number) => {
    setSelection(null)
    changeLimit(next)
  }

  const changeOrder = (next: SortKey[]) => {
    setSelection(null)
    changeSort(next)
  }

  /**
   * The conditions travel into the read's own `WHERE`, so a filter narrows the whole table
   * rather than the page in front — and the panel that spells them opens with the first one.
   */
  const changeConditions = (next: Group) => {
    setSelection(null)
    setFilterShown(next.children.length > 0 || filterShown)
    changeCriteria(next)
  }

  /**
   * The values a column holds, for the filter builder's picker. Held steady across renders,
   * because the picker reads it once per column and one rebuilt every render would never stop.
   */
  const suggest = useMemo(
    () => (column: string) =>
      tables.columnValues(sessionId, {
        database: namespace.database,
        schema: namespace.schema,
        table,
        column,
      }),
    [sessionId, namespace.database, namespace.schema, table],
  )

  /**
   * Lays a value out for the value viewer — the engine's own `Format`, since a document store's
   * documents are its own notation and not JSON. Held steady across renders for the reason
   * `suggest` is: the viewer calls it from an effect, and one rebuilt every render would never
   * settle. A SQL engine never types a column `object`, so on one it is simply never called.
   */
  const formatValue = useMemo(
    () => (value: string) => queries.format(sessionId, value),
    [sessionId],
  )

  /** The panel edits the focused cell, so the menu's own cell becomes the selection first. */
  const editInPanel = (ref: CellRef) => {
    setSelection(oneRange({ anchor: ref, focus: ref }))
    change(tabId, { panels: new Set(panels).add('cell') })
  }

  /**
   * The page in front, with the columns that are showing — a table's rows are paged by the
   * server, so what the grid holds *is* the page, and a hidden column is one nobody asked for.
   */
  const exportRows = (format: ExportFormat) =>
    void rowExport.run(sessionId, format, {
      namespace,
      table,
      columns: grid.columns.map((column) => column.name),
      rows: grid.rows,
    })

  const failure = edits.error ?? (data ? error : null)

  /**
   * Which panels are open, named for the splitter. A layout is remembered per *combination*, which
   * is what `panelIds` is for: the width the reader gave the column list is theirs the next time
   * they open it, and closing it does not teach the grid a new size.
   */
  const rowPanels = useMemo(() => (panels.has('cell') ? ['rows', 'cell'] : ['rows']), [panels])
  const gridPanels = useMemo(() => (panels.has('column') ? ['picker', 'grid'] : ['grid']), [panels])

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <TableToolbar
        active={panels}
        onToggle={togglePanel}
        filterShown={filterShown}
        filterCount={countConditions(criteria) + sort.length}
        onFilterToggle={() => setFilterShown(!filterShown)}
        onExport={data !== null ? exportRows : undefined}
      />

      {/* Both side panels are dragged rather than fixed, and the handle between them is the only
          divider — a border of their own beside it would read as two lines. */}
      <Split
        id="vibe-db.table.rows"
        orientation="vertical"
        panelIds={rowPanels}
        className="min-h-0 flex-1"
      >
        <SplitPanel id="rows" minSize="25%">
          <Split
            id="vibe-db.table.columns"
            orientation="horizontal"
            panelIds={gridPanels}
            className="min-h-0"
          >
            {panels.has('column') ? (
              <>
                <SplitPanel
                  id="picker"
                  defaultSize={PICKER_WIDTH}
                  minSize={PICKER_MIN_WIDTH}
                  maxSize={PICKER_MAX_WIDTH}
                >
                  <ColumnPicker
                    columns={arranged.map((one) => one.column)}
                    hidden={hidden}
                    onToggle={toggleColumn}
                  />
                </SplitPanel>

                <SplitHandle orientation="horizontal" />
              </>
            ) : null}

            <SplitPanel id="grid" className="relative min-w-0">
              {/* A read of every row arrives in batches and is watched rather than waited on, so
                  its count and its Stop sit over whatever the grid already holds. */}
              {fetch !== null ? <FetchProgress fetched={fetch.fetched} onStop={stopFetch} /> : null}

              {/* Nothing has arrived yet: a grid with no columns is a blank pane, and the first page
                  of a large table is exactly where the reader waits longest. Once rows are on screen
                  a re-read leaves them there and the footer's pager says it is working. */}
              {data === null ? (
                error ? (
                  <Empty title={error.message} hint={error.detail} />
                ) : fetch === null ? (
                  // A streamed read reports itself, and two spinners over one another say no more.
                  <Loading label={`Loading ${table}…`} />
                ) : null
              ) : (
                <ResultGrid
                  result={grid}
                  selection={selection}
                  onSelectionChange={setSelection}
                  onEditCell={
                    editable
                      ? (ref: CellRef, value) =>
                          edits.setCell({ row: ref.row, column: realColumn(ref.column) }, value)
                      : undefined
                  }
                  isDirty={(row, column) => edits.isDirty(row, realColumn(column))}
                  isDeleted={(row) => edits.deleted.has(row)}
                  sort={sort}
                  onSortChange={changeOrder}
                  onMoveColumn={moveColumn}
                  // Export and Filter & Sort live in this surface's own toolbar, so the grid draws
                  // no strip of its own — but the cell menu still reaches the same handlers.
                  onExport={exportRows}
                  headStrip={false}
                  onEditInPanel={editInPanel}
                  onDeleteRows={editable ? (rows) => edits.deleteRows(rows) : undefined}
                  onRefresh={refresh}
                  formatValue={formatValue}
                  filtering={{
                    criteria,
                    shown: filterShown,
                    onShownChange: setFilterShown,
                    onChange: changeConditions,
                    suggest,
                  }}
                  columnMenuItems={(column) => [
                    {
                      label: 'Hide column',
                      icon: ICON.hide,
                      separatorBefore: true,
                      disabled: visible.length === 1,
                      onSelect: () => toggleColumn(column.name),
                    },
                    {
                      label: 'Show all columns',
                      icon: ICON.show,
                      disabled: hidden.size === 0,
                      onSelect: showAllColumns,
                    },
                  ]}
                />
              )}
            </SplitPanel>
          </Split>
        </SplitPanel>

        {panels.has('cell') ? (
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
                      : 'This table has no primary key, so a row cannot be identified for editing.'
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

      {failure !== null ? <ErrorStrip error={failure} /> : null}

      <GridFooter
        editable={editable}
        selected={selection !== null}
        pending={edits.pending}
        busy={edits.saving || loading}
        onAddRow={addRow}
        onDeleteRow={() => edits.deleteRows(selectedRows(selection))}
        onApply={() => void edits.apply()}
        onDiscard={edits.discard}
        onRefresh={refresh}
      >
        <GridPager
          page={page}
          size={limit}
          total={data?.total ?? 0}
          loading={loading}
          onPageChange={changePage}
          onSizeChange={changePageSize}
        />
      </GridFooter>

      <GridStatus
        statement={data?.statement ?? ''}
        record={record}
        total={data?.total ?? 0}
        page={page + 1}
        pageCount={pageCount(limit, data?.total ?? 0)}
        loadedAt={loadedAt}
      />
    </div>
  )
}

/** The same set with one member in or out, which is what every toggle in this view means. */
function toggled<T>(members: ReadonlySet<T>, one: T): ReadonlySet<T> {
  const next = new Set(members)
  if (!next.delete(one)) next.add(one)
  return next
}

function sourceOf(data: TablePage | null, namespace: Namespace, table: string): EditSource | null {
  if (data === null) return null

  return {
    target: { database: namespace.database, schema: namespace.schema, table },
    columns: data.columns,
    rows: data.rows,
  }
}
