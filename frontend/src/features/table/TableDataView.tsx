import { useMemo, useState } from 'react'

import type { Namespace } from '@/bridge/connections'
import { ResultGrid } from '@/features/results/ResultGrid'
import type { CellRange } from '@/features/results/resultSet'
import { Empty } from '@/shared/ui/Empty'

import { CellEditor } from './CellEditor'
import { ColumnPicker } from './ColumnPicker'
import { TablePager } from './TablePager'
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
  const { data, loading, saving, error, page, limit, goToPage, changeLimit, refresh, updateCell } =
    useTableData(sessionId, namespace, table)

  const [panels, setPanels] = useState<ReadonlySet<PanelKind>>(new Set())
  const [hidden, setHidden] = useState<ReadonlySet<string>>(new Set())
  const [selection, setSelection] = useState<CellRange | null>(null)

  const columns = data?.columns ?? []
  const rows = data?.rows ?? []
  const editable = columns.some((column) => column.primaryKey)

  const visible = useMemo(
    () => columns.flatMap((column, index) => (hidden.has(column.name) ? [] : [{ column, index }])),
    [columns, hidden],
  )

  const grid = useMemo(
    () => ({
      columns: visible.map((entry) => entry.column),
      rows: rows.map((row) => visible.map((entry) => row[entry.index] ?? null)),
    }),
    [rows, visible],
  )

  const target = useMemo(() => {
    if (!selection) return null

    const entry = visible[selection.anchor.column]
    const row = rows[selection.anchor.row]
    if (!entry || !row) return null

    return {
      ref: { row: selection.anchor.row, column: entry.index },
      column: entry.column,
      value: row[entry.index] ?? null,
    }
  }, [selection, visible, rows])

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

  const changePage = (next: number) => {
    setSelection(null)
    goToPage(next)
  }

  const changePageSize = (next: number) => {
    setSelection(null)
    changeLimit(next)
  }

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
            <ResultGrid result={grid} selection={selection} onSelectionChange={setSelection} />
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
              busy={saving}
              onApply={(value) => void updateCell(target.ref, value)}
              onRefresh={refresh}
            />
          )}
        </div>
      ) : null}

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
