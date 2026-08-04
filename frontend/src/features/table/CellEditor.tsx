import { useState } from 'react'

import type { Cell, TableColumn } from '@/bridge/tables'
import { ValueViewer } from '@/features/results/ValueViewer'
import { Button } from '@/shared/ui/Button'

interface CellEditorProps {
  column: TableColumn
  value: Cell
  editable: boolean
  busy: boolean
  /** Stages the value on the cell, exactly as the grid's own editor does. */
  onApply: (value: Cell) => void
  /** The engine's own layout, for a value that is not JSON — see `ValueViewer.formatValue`. */
  formatValue?: ((value: string) => Promise<string>) | undefined
}

export function CellEditor({
  column,
  value,
  editable,
  busy,
  onApply,
  formatValue,
}: CellEditorProps) {
  const [draft, setDraft] = useState(value ?? '')
  const [isNull, setIsNull] = useState(value === null)

  const next: Cell = isNull ? null : draft
  const dirty = next !== value

  const discard = () => {
    setDraft(value ?? '')
    setIsNull(value === null)
  }

  return (
    <div className="flex h-full min-h-0 flex-col bg-canvas">
      <div className="flex h-8 shrink-0 items-center gap-2 border-b border-line bg-panel px-2">
        <span className="truncate text-xs font-medium text-fg">{column.name}</span>
        <span className="shrink-0 font-mono text-2xs text-faint uppercase">{column.fullType}</span>
        {column.primaryKey ? (
          <span className="shrink-0 rounded bg-accent-subtle px-1 font-mono text-2xs text-accent">
            PK
          </span>
        ) : null}

        <div className="min-w-2 flex-1" />

        {column.nullable ? (
          <label className="flex shrink-0 items-center gap-1 text-xs text-muted">
            <input
              type="checkbox"
              checked={isNull}
              disabled={!editable || busy}
              onChange={(e) => setIsNull(e.target.checked)}
              className="accent-[var(--accent)]"
            />
            NULL
          </label>
        ) : null}

        <Button onClick={discard} disabled={!dirty || busy}>
          Discard
        </Button>
        <Button
          variant="primary"
          onClick={() => onApply(next)}
          disabled={!dirty || !editable || busy}
        >
          Apply
        </Button>
      </div>

      {/* The panel shows the *draft*, so a JSON document being typed is laid out as it is typed
          and a `\x…` blob is still a hex dump after the cell beside it was changed. Only Raw is
          editable — see ValueViewer. */}
      <ValueViewer
        className="min-h-0 flex-1"
        value={isNull ? null : draft}
        columnType={column.fullType || column.type}
        onChange={editable && !isNull ? setDraft : undefined}
        formatValue={formatValue}
      />

      {editable ? null : (
        <p className="shrink-0 border-t border-line px-3 py-1 text-xs text-warn">
          This table has no primary key, so a row cannot be identified for editing.
        </p>
      )}
    </div>
  )
}
