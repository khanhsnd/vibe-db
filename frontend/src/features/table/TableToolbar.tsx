import { cn } from '@/lib/cn'

export type PanelKind = 'cell' | 'column'

const ITEMS: ReadonlyArray<{ kind: PanelKind; label: string }> = [
  { kind: 'cell', label: 'Cell Editor' },
  { kind: 'column', label: 'Column' },
]

interface TableToolbarProps {
  active: PanelKind | null
  onToggle: (kind: PanelKind) => void
}

export function TableToolbar({ active, onToggle }: TableToolbarProps) {
  return (
    <div
      role="toolbar"
      className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-panel px-2"
    >
      {ITEMS.map((item) => (
        <button
          key={item.kind}
          type="button"
          aria-pressed={active === item.kind}
          onClick={() => onToggle(item.kind)}
          className={cn(
            'inline-flex h-6 items-center rounded-md px-2 text-xs font-medium transition-colors',
            active === item.kind
              ? 'bg-selected text-fg'
              : 'text-muted hover:bg-hover hover:text-fg',
          )}
        >
          {item.label}
        </button>
      ))}
    </div>
  )
}
