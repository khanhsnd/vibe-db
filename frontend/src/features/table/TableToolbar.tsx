import { Columns3, PanelBottom } from 'lucide-react'
import type { ReactNode } from 'react'

import { cn } from '@/lib/cn'

export type PanelKind = 'cell' | 'column'

const ITEMS: ReadonlyArray<{ kind: PanelKind; label: string; icon: ReactNode }> = [
  { kind: 'cell', label: 'Cell Editor', icon: <PanelBottom size={13} /> },
  { kind: 'column', label: 'Column', icon: <Columns3 size={13} /> },
]

interface TableToolbarProps {
  active: ReadonlySet<PanelKind>
  onToggle: (kind: PanelKind) => void
}

export function TableToolbar({ active, onToggle }: TableToolbarProps) {
  return (
    <div
      role="toolbar"
      className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-bar px-2"
    >
      {ITEMS.map((item) => (
        <button
          key={item.kind}
          type="button"
          aria-pressed={active.has(item.kind)}
          onClick={() => onToggle(item.kind)}
          className={cn(
            'inline-flex h-6 items-center gap-1.5 rounded-md px-2 text-xs font-medium transition-colors',
            active.has(item.kind)
              ? 'bg-selected text-fg'
              : 'text-muted hover:bg-hover hover:text-fg',
          )}
        >
          {item.icon}
          {item.label}
        </button>
      ))}
    </div>
  )
}
