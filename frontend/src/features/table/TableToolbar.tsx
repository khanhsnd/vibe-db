import { Columns3, PanelBottom } from 'lucide-react'
import type { ReactNode } from 'react'

import type { ExportFormat } from '@/bridge/export'
import { ExportMenu } from '@/features/results/ExportMenu'
import { cn } from '@/lib/cn'
import { Button } from '@/shared/ui/Button'

export type PanelKind = 'cell' | 'column'

const ITEMS: ReadonlyArray<{ kind: PanelKind; label: string; icon: ReactNode }> = [
  { kind: 'cell', label: 'Cell Editor', icon: <PanelBottom size={14} /> },
  { kind: 'column', label: 'Column', icon: <Columns3 size={14} /> },
]

interface TableToolbarProps {
  active: ReadonlySet<PanelKind>
  onToggle: (kind: PanelKind) => void
  /** Left out until there are rows to write, which is what keeps Export off an empty table. */
  onExport?: ((format: ExportFormat) => void) | undefined
}

/**
 * The one strip over a table's grid: the panels a reader can open on the left, and Export against
 * the right, where every grid keeps it. The grid draws no head strip of its own beneath this one.
 */
export function TableToolbar({ active, onToggle, onExport }: TableToolbarProps) {
  return (
    <div
      role="toolbar"
      className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-bar px-1.5"
    >
      {ITEMS.map((item) => (
        <Button
          key={item.kind}
          variant="subtle"
          icon={item.icon}
          aria-pressed={active.has(item.kind)}
          onClick={() => onToggle(item.kind)}
          className={cn(active.has(item.kind) && 'bg-selected text-fg')}
        >
          {item.label}
        </Button>
      ))}

      <div className="min-w-2 flex-1" />

      {onExport ? <ExportMenu onExport={onExport} /> : null}
    </div>
  )
}
