import { useState, type ReactNode } from 'react'

import { cn } from '@/lib/cn'

export interface ResultPaneDef {
  id: string
  label: string
  content: ReactNode
}

/**
 * The tab strip over the output panel. A further pane — messages, profile, history —
 * is one more entry in the list the caller passes; nothing here needs to change.
 */
export function ResultTabs({ panes }: { panes: ResultPaneDef[] }) {
  const [selectedId, setSelectedId] = useState(panes[0]?.id ?? '')
  const active = panes.find((pane) => pane.id === selectedId) ?? panes[0]

  return (
    <div className="flex h-full min-h-0 flex-col">
      <div
        role="tablist"
        className="flex h-8 shrink-0 items-end gap-0.5 border-b border-line bg-panel px-1"
      >
        {panes.map((pane) => (
          <button
            key={pane.id}
            type="button"
            role="tab"
            aria-selected={pane.id === active?.id}
            onClick={() => setSelectedId(pane.id)}
            className={cn(
              'h-7 rounded-t-md border border-b-0 px-3 text-xs font-medium transition-colors',
              pane.id === active?.id
                ? 'border-line bg-canvas text-fg'
                : 'border-transparent text-muted hover:bg-hover hover:text-fg',
            )}
          >
            {pane.label}
          </button>
        ))}
      </div>

      <div className="min-h-0 flex-1">{active?.content}</div>
    </div>
  )
}
