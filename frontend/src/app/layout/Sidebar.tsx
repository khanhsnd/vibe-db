import { Plus, Search } from 'lucide-react'

import { ObjectTree } from '@/features/connections/ObjectTree'
import type { TreeNode } from '@/features/connections/tree'
import type { MenuItem } from '@/shared/ui/ContextMenu'
import { Empty } from '@/shared/ui/Empty'
import { IconButton } from '@/shared/ui/IconButton'

interface SidebarProps {
  roots: TreeNode[]
  expanded: ReadonlySet<string>
  selectedId: string | null
  filter: string
  onFilterChange: (value: string) => void
  onToggle: (id: string) => void
  onSelect: (node: TreeNode) => void
  onActivate: (node: TreeNode) => void
  onNewConnection: () => void
  menuItemsFor: (node: TreeNode) => MenuItem[]
}

export function Sidebar({
  roots,
  expanded,
  selectedId,
  filter,
  onFilterChange,
  onToggle,
  onSelect,
  onActivate,
  onNewConnection,
  menuItemsFor,
}: SidebarProps) {
  return (
    <aside className="flex h-full min-h-0 flex-col bg-panel">
      <div className="flex h-8 shrink-0 items-center gap-1 pr-1 pl-2.5">
        <span className="text-[10.5px] font-semibold tracking-[0.08em] text-faint uppercase">
          Connections
        </span>
        <div className="flex-1" />
        <IconButton label="New connection" icon={<Plus size={14} />} onClick={onNewConnection} />
      </div>

      <div className="shrink-0 px-2 pb-2">
        <div className="flex h-7 items-center gap-1.5 rounded-md border border-line bg-canvas px-2 focus-within:border-accent">
          <Search size={12} className="shrink-0 text-faint" />
          <input
            value={filter}
            onChange={(e) => onFilterChange(e.target.value)}
            placeholder="Filter objects"
            spellCheck={false}
            className="min-w-0 flex-1 bg-transparent text-[12px] text-fg outline-none placeholder:text-faint"
          />
        </div>
      </div>

      <div className="min-h-0 flex-1">
        {roots.length === 0 ? (
          <Empty title="No connections yet" hint="Create one to get started." />
        ) : (
          <ObjectTree
            roots={roots}
            expanded={expanded}
            selectedId={selectedId}
            onToggle={onToggle}
            onSelect={onSelect}
            onActivate={onActivate}
            menuItemsFor={menuItemsFor}
          />
        )}
      </div>
    </aside>
  )
}
