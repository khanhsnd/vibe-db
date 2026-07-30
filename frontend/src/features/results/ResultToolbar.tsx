import { Check, RotateCw, Table2, Undo2 } from 'lucide-react'

import { Button } from '@/shared/ui/Button'

interface ResultToolbarProps {
  label: string
  editable: boolean
  /** The buttons only make sense once a row is picked, so they appear with the selection. */
  selected: boolean
  dirty: boolean
  busy: boolean
  onDiscard: () => void
  onApply: () => void
  onRefresh: () => void
}

export function ResultToolbar({
  label,
  editable,
  selected,
  dirty,
  busy,
  onDiscard,
  onApply,
  onRefresh,
}: ResultToolbarProps) {
  return (
    <div
      role="toolbar"
      className="flex h-9 shrink-0 items-center gap-1.5 border-b border-line bg-panel px-2"
    >
      <Table2 size={13} className="shrink-0 text-faint" />
      <span className="truncate font-mono text-xs text-muted">{label}</span>

      <div className="min-w-2 flex-1" />

      {selected ? (
        <>
          <Button icon={<Undo2 size={12} />} disabled={!dirty || busy} onClick={onDiscard}>
            Discard
          </Button>
          <Button
            variant="primary"
            icon={<Check size={12} />}
            disabled={!dirty || !editable || busy}
            onClick={onApply}
          >
            Apply
          </Button>
          <Button
            variant="subtle"
            icon={<RotateCw size={12} />}
            disabled={busy}
            onClick={onRefresh}
          >
            Refresh
          </Button>
        </>
      ) : null}
    </div>
  )
}
