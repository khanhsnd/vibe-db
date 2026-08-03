import { useMemo, useState } from 'react'

import type { Namespace } from '@/bridge/connections'
import { rowExport, type ExportFormat } from '@/bridge/export'
import type { TablePage } from '@/bridge/tables'
import { GridFooter } from '@/features/results/GridFooter'
import { GridPager } from '@/features/results/GridPager'
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
import { ICON } from '@/shared/ui/menuIcons'

import { CellEditor } from './CellEditor'
import { ColumnPicker } from './ColumnPicker'
import { TableToolbar, type PanelKind } from './TableToolbar'
import { useTableData } from './useTableData'

const EDITOR_HEIGHT = 180
const PICKER_WIDTH = 168

interface TableDataViewProps {
  sessionId: string
  namespace: Namespace
  table: string
}

export function TableDataView({ sessionId, namespace, table }: TableDataViewProps) {
  const { data, loading, error, page, limit, sort, goToPage, changeLimit, changeSort, refresh } =
    useTableData(sessionId, namespace, table)

  const [panels, setPanels] = useState<ReadonlySet<PanelKind>>(new Set())
  const [hidden, setHidden] = useState<ReadonlySet<string>>(new Set())
  const [selection, setSelection] = useState<Selection | null>(null)

  const columns = data?.columns ?? []
  const editable = columns.some((column) => column.primaryKey)
  const edits = useGridEdits(sourceOf(data, namespace, table), sessionId, refresh)

  const visible = useMemo(
    () => columns.flatMap((column, index) => (hidden.has(column.name) ? [] : [{ column, index }])),
    [columns, hidden],
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

  const togglePanel = (kind: PanelKind) =>
    setPanels((current) => {
      const next = new Set(current)
      if (next.has(kind)) next.delete(kind)
      else next.add(kind)
      return next
    })

  const toggleColumn = (name: string) => {
    setSelection(null)
    setHidden((current) => {
      const next = new Set(current)
      if (next.has(name)) next.delete(name)
      else next.add(name)
      return next
    })
  }

  /** The draft lands after every fetched row, and is selected so a paste that follows means it. */
  const addRow = () => {
    setSelection(oneRange(rowRange(edits.rows.length, visible.length)))
    edits.addRow()
  }

  const showAllColumns = () => {
    setSelection(null)
    setHidden(new Set())
  }

  const changePage = (next: number) => {
    setSelection(null)
    goToPage(next)
  }

  const changePageSize = (next: number) => {
    setSelection(null)
    changeLimit(next)
  }

  const changeOrder = (next: SortKey | null) => {
    setSelection(null)
    changeSort(next)
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

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <TableToolbar active={panels} onToggle={togglePanel} />

      <div className="flex min-h-0 flex-1">
        {panels.has('column') ? (
          <div className="shrink-0 border-r border-line" style={{ width: PICKER_WIDTH }}>
            <ColumnPicker columns={columns} hidden={hidden} onToggle={toggleColumn} />
          </div>
        ) : null}

        <div className="min-w-0 flex-1">
          {error && !data ? (
            <Empty title={error.message} hint={error.detail} />
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
              onExport={data !== null ? exportRows : undefined}
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
        </div>
      </div>

      {panels.has('cell') ? (
        <div className="shrink-0 border-t border-line" style={{ height: EDITOR_HEIGHT }}>
          {target === null ? (
            <Empty title="No cell selected" hint="Click a cell in the grid to edit its value." />
          ) : (
            <CellEditor
              key={`${target.ref.row}:${target.ref.column}`}
              column={target.column}
              value={target.value}
              editable={editable}
              busy={edits.saving}
              onApply={(value) => edits.setCell(target.ref, value)}
            />
          )}
        </div>
      ) : null}

      {failure !== null ? (
        <p className="shrink-0 truncate border-t border-danger/40 bg-danger-subtle px-2 py-1 text-xs text-danger">
          {failure.message}
        </p>
      ) : null}

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
    </div>
  )
}

function sourceOf(data: TablePage | null, namespace: Namespace, table: string): EditSource | null {
  if (data === null) return null

  return {
    target: { database: namespace.database, schema: namespace.schema, table },
    columns: data.columns,
    rows: data.rows,
  }
}
