import { Eye } from 'lucide-react'

import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'

import type { CellRef, GridData } from './resultSet'
import { ValueViewer } from './ValueViewer'

/** Tall enough for a hex dump or a shape to be worth opening, and short of the dialog's ceiling. */
const HEIGHT = 380

/**
 * One cell read whole. It is a dialog rather than a panel because **every** grid raises it — a
 * query result and the history log have no cell-editor panel to put it in, and a JSON document in
 * a query result is exactly the thing a reader most often wants to unfold.
 *
 * It is read-only whatever grid it was raised from: a cell is *written* where it is written today,
 * in the grid or in the cell-editor panel, and a second way in that also wrote would be a second
 * set of pending edits to reconcile.
 */
export function ValueDialog({
  at,
  data,
  onClose,
}: {
  /** The cell being read, or null when nothing is. */
  at: CellRef | null
  data: GridData
  onClose: () => void
}) {
  const column = at ? data.columns[at.column] : undefined
  const value = at ? (data.rows[at.row]?.[at.column] ?? null) : null

  return (
    <Dialog
      open={at !== null && column !== undefined}
      onOpenChange={(open) => {
        if (!open) onClose()
      }}
      title={column?.name ?? ''}
      description={column?.fullType || column?.type || ''}
      icon={<Eye size={15} />}
      width={560}
      bodyClassName=""
      footer={
        <>
          <div className="flex-1" />
          <Button variant="default" onClick={onClose}>
            Close
          </Button>
        </>
      }
    >
      <div style={{ height: HEIGHT }}>
        <ValueViewer
          className="h-full"
          value={value}
          columnType={column?.fullType || column?.type}
        />
      </div>
    </Dialog>
  )
}
