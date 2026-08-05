import { useState } from 'react'

import type { Cell } from '@/bridge/tables'
import { Button } from '@/shared/ui/Button'

import type { ResultColumn } from './resultSet'
import { ValueViewer } from './ValueViewer'

/**
 * What the panel opens at, and how far it may be dragged. Only the size it opens at is a look — the
 * bounds are what stop a drag leaving a panel too small to read: the floor is the panel's own
 * chrome, a 32 px header and the viewer's 28 px strip, and enough left to read one line of value.
 */
export const EDITOR_HEIGHT = 180
export const EDITOR_MIN_HEIGHT = 120

interface CellEditorProps {
  column: ResultColumn
  value: Cell
  /**
   * Why the value cannot be written back, where it cannot — a line under the panel says so, and its
   * absence is what makes the panel writable. The reason is the *surface's*: a table with no primary
   * key and a result no `UPDATE` can name are not the same fact.
   */
  readOnly?: string | undefined
  busy: boolean
  /** Stages the value on the cell, exactly as the grid's own editor does. */
  onApply: (value: Cell) => void
  /** The engine's own layout, for a value that is not JSON — see `ValueViewer.formatValue`. */
  formatValue?: ((value: string) => Promise<string>) | undefined
}

export function CellEditor({
  column,
  value,
  readOnly,
  busy,
  onApply,
  formatValue,
}: CellEditorProps) {
  const [draft, setDraft] = useState(value ?? '')
  const [isNull, setIsNull] = useState(value === null)

  const editable = readOnly === undefined
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
        <span className="shrink-0 font-mono text-2xs text-faint uppercase">
          {column.fullType || column.type}
        </span>
        {column.primaryKey === true ? (
          <span className="shrink-0 rounded bg-accent-subtle px-1 font-mono text-2xs text-accent">
            PK
          </span>
        ) : null}

        <div className="min-w-2 flex-1" />

        {/* Offered on every column, as the cell menu's own Set to NULL is: a driver that cannot
            report nullability says `false`, and a tick left off for that reason is a value the
            reader cannot reach. */}
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
          and a `\x…` blob is still a hex dump after the cell beside it was changed. Every view
          that is text is typed into — see ValueViewer. */}
      <ValueViewer
        className="min-h-0 flex-1"
        value={isNull ? null : draft}
        columnType={column.fullType || column.type}
        onChange={editable && !isNull ? setDraft : undefined}
        formatValue={formatValue}
      />

      {readOnly === undefined ? null : (
        <p className="shrink-0 border-t border-line px-3 py-1 text-xs text-warn">{readOnly}</p>
      )}
    </div>
  )
}
