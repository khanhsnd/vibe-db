import { useState } from 'react'

import type { Cell, TableColumn } from '@/bridge/tables'
import { ResultGrid } from '@/features/results/ResultGrid'
import type { CellRef } from '@/features/results/resultSet'
import { Empty } from '@/shared/ui/Empty'

import { CellEditor } from './CellEditor'
import { ColumnList } from './ColumnList'
import { TablePager } from './TablePager'
import { TableToolbar, type PanelKind } from './TableToolbar'
import { useTableData } from './useTableData'

const PANEL_HEIGHT = 180

interface TableDataViewProps {
  sessionId: string
  database: string
  table: string
}

export function TableDataView({ sessionId, database, table }: TableDataViewProps) {
  const { data, loading, saving, error, page, limit, goToPage, changeLimit, refresh, updateCell } =
    useTableData(sessionId, database, table)

  const [panel, setPanel] = useState<PanelKind | null>(null)
  const [selected, setSelected] = useState<CellRef | null>(null)

  const columns = data?.columns ?? []
  const rows = data?.rows ?? []
  const editable = columns.some((column) => column.primaryKey)
  const selection = resolveSelection(selected, columns, rows)

  const changePage = (next: number) => {
    setSelected(null)
    goToPage(next)
  }

  const changePageSize = (next: number) => {
    setSelected(null)
    changeLimit(next)
  }

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <TableToolbar
        active={panel}
        onToggle={(kind) => setPanel((current) => (current === kind ? null : kind))}
      />

      <div className="min-h-0 flex-1">
        {error && !data ? (
          <Empty title={error.message} hint={error.detail} />
        ) : (
          <ResultGrid result={{ columns, rows }} selected={selected} onSelect={setSelected} />
        )}
      </div>

      {panel === null ? null : (
        <div className="shrink-0 border-t border-line" style={{ height: PANEL_HEIGHT }}>
          {panel === 'column' ? (
            <ColumnList columns={columns} />
          ) : selection === null ? (
            <Empty title="No cell selected" hint="Click a cell in the grid to edit its value." />
          ) : (
            <CellEditor
              key={`${selection.ref.row}:${selection.ref.column}`}
              column={selection.column}
              value={selection.value}
              editable={editable}
              busy={saving}
              onApply={(value) => void updateCell(selection.ref, value)}
              onRefresh={refresh}
            />
          )}
        </div>
      )}

      <TablePager
        page={page}
        limit={limit}
        total={data?.total ?? 0}
        loading={loading}
        error={data ? error : null}
        onPageChange={changePage}
        onLimitChange={changePageSize}
      />
    </div>
  )
}

interface Selection {
  ref: CellRef
  column: TableColumn
  value: Cell
}

function resolveSelection(
  ref: CellRef | null,
  columns: TableColumn[],
  rows: Cell[][],
): Selection | null {
  if (ref === null) return null

  const column = columns[ref.column]
  const row = rows[ref.row]
  if (!column || !row) return null

  return { ref, column, value: row[ref.column] ?? null }
}
