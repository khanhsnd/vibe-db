import type { TableColumn } from '@/bridge/tables'

const HEADERS = ['Name', 'Type', 'Nullable', 'Key', 'Default'] as const

const CELL = 'border-b border-line/60 px-2 py-1 align-top'
const MONO = `${CELL} font-mono text-xs text-muted`

export function ColumnList({ columns }: { columns: TableColumn[] }) {
  return (
    <div className="scroll-thin h-full overflow-auto bg-canvas">
      <table className="w-full border-collapse text-sm">
        <thead className="sticky top-0 z-1 bg-panel">
          <tr>
            {HEADERS.map((header) => (
              <th
                key={header}
                className="border-b border-line-strong px-2 py-1 text-left text-xs font-medium text-muted"
              >
                {header}
              </th>
            ))}
          </tr>
        </thead>

        <tbody>
          {columns.map((column) => (
            <tr key={column.name} className="hover:bg-hover">
              <td className={`${CELL} text-fg`}>{column.name}</td>
              <td className={MONO}>{column.fullType}</td>
              <td className={`${CELL} text-xs text-muted`}>{column.nullable ? 'YES' : 'NO'}</td>
              <td className={MONO}>{column.primaryKey ? 'PRI' : ''}</td>
              <td className={MONO}>{column.default ?? ''}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  )
}
