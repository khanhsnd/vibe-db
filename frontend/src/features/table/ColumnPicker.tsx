import type { TableColumn } from '@/bridge/tables'

interface ColumnPickerProps {
  columns: TableColumn[]
  hidden: ReadonlySet<string>
  onToggle: (name: string) => void
}

export function ColumnPicker({ columns, hidden, onToggle }: ColumnPickerProps) {
  const onlyOneLeft = columns.filter((column) => !hidden.has(column.name)).length === 1

  return (
    <div className="scroll-thin h-full overflow-auto bg-panel py-1">
      {columns.map((column) => {
        const checked = !hidden.has(column.name)

        return (
          <label
            key={column.name}
            className="flex h-6 cursor-default items-center gap-2 px-2 text-sm text-fg hover:bg-hover"
          >
            <input
              type="checkbox"
              checked={checked}
              disabled={checked && onlyOneLeft}
              onChange={() => onToggle(column.name)}
              className="shrink-0 accent-[var(--accent)]"
            />
            <span className="truncate">{column.name}</span>
          </label>
        )
      })}
    </div>
  )
}
