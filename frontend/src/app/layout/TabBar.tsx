import { Plus, Table2, Terminal, X } from 'lucide-react'

import { cn } from '@/lib/cn'
import { IconButton } from '@/shared/ui/IconButton'
import { useTabsStore, type Tab } from '@/stores/tabs'

const MIDDLE_BUTTON = 1

export function TabBar({ onNewQuery }: { onNewQuery: () => void }) {
  const tabs = useTabsStore((s) => s.tabs)
  const activeId = useTabsStore((s) => s.activeId)
  const activate = useTabsStore((s) => s.activate)
  const close = useTabsStore((s) => s.close)

  return (
    <div
      role="tablist"
      className="scroll-thin flex h-8 shrink-0 items-stretch overflow-x-auto border-b border-line bg-panel"
    >
      {tabs.map((tab) => (
        <TabItem
          key={tab.id}
          tab={tab}
          active={tab.id === activeId}
          onSelect={() => activate(tab.id)}
          onClose={() => close(tab.id)}
        />
      ))}

      <div className="flex items-center px-1">
        <IconButton label="New query" icon={<Plus size={14} />} onClick={onNewQuery} />
      </div>
    </div>
  )
}

interface TabItemProps {
  tab: Tab
  active: boolean
  onSelect: () => void
  onClose: () => void
}

function TabItem({ tab, active, onSelect, onClose }: TabItemProps) {
  const Icon = tab.kind === 'query' ? Terminal : Table2

  return (
    <div
      role="tab"
      aria-selected={active}
      onClick={onSelect}
      onAuxClick={(e) => {
        if (e.button === MIDDLE_BUTTON) onClose()
      }}
      className={cn(
        'group relative flex max-w-[220px] min-w-0 shrink-0 cursor-default items-center gap-1.5',
        'border-r border-line px-3 text-sm whitespace-nowrap transition-colors',
        active ? 'bg-canvas text-fg' : 'text-muted hover:bg-hover hover:text-fg',
      )}
    >
      {active ? <span className="absolute inset-x-0 top-0 h-0.5 bg-accent" /> : null}

      <Icon size={13} className="shrink-0 text-faint" />
      <span className="truncate">{tab.title}</span>

      <button
        type="button"
        aria-label={`Close ${tab.title}`}
        onClick={(e) => {
          e.stopPropagation()
          onClose()
        }}
        className={cn(
          'ml-0.5 grid h-4 w-4 shrink-0 place-items-center rounded',
          'text-faint opacity-0 hover:bg-hover hover:text-fg',
          'group-hover:opacity-100 focus-visible:opacity-100',
          active && 'opacity-60',
        )}
      >
        <X size={11} />
      </button>
    </div>
  )
}
