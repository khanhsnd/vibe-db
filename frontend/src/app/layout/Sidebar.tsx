import { Plus, Search, SearchX, Server, X } from 'lucide-react'

import { ObjectTree } from '@/features/connections/ObjectTree'
import type { PickHow, TreeNode } from '@/features/connections/tree'
import { Button } from '@/shared/ui/Button'
import type { MenuItem } from '@/shared/ui/ContextMenu'
import { Empty } from '@/shared/ui/Empty'
import { IconButton } from '@/shared/ui/IconButton'

interface SidebarProps {
  roots: TreeNode[]
  expanded: ReadonlySet<string>
  /** Every row picked, and the focused one — see `ObjectTree`, which draws the two differently. */
  pickedIds: ReadonlySet<string>
  focusedId: string | null
  filter: string
  onFilterChange: (value: string) => void
  /** What each connection draws of its databases, which is the tree's own control — see `ObjectTree`. */
  databaseFilters: Record<string, string>
  onToggle: (node: TreeNode) => void
  onSelect: (node: TreeNode, how: PickHow) => void
  onActivate: (node: TreeNode) => void
  onDatabaseFilter: (profileId: string, pattern: string) => void
  onNewConnection: () => void
  menuItemsFor: (node: TreeNode) => MenuItem[]
}

export function Sidebar({
  roots,
  expanded,
  pickedIds,
  focusedId,
  filter,
  onFilterChange,
  databaseFilters,
  onToggle,
  onSelect,
  onActivate,
  onDatabaseFilter,
  onNewConnection,
  menuItemsFor,
}: SidebarProps) {
  return (
    <aside className="flex h-full min-h-0 flex-col bg-panel">
      {/* The filter is the whole of the panel's chrome: a title over a tree of servers would say
          what the tree already says, and the sidebar is 240 px of height worth spending on names. */}
      <div className="flex h-8 shrink-0 items-center gap-1 border-b border-line bg-bar pr-1 pl-1.5">
        <div className="flex h-6 min-w-0 flex-1 items-center gap-1.5 rounded-md border border-line bg-canvas px-1.5 transition-colors focus-within:border-accent focus-within:ring-1 focus-within:ring-accent">
          <Search size={12} className="shrink-0 text-faint" />
          <input
            value={filter}
            onChange={(e) => onFilterChange(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === 'Escape') onFilterChange('')
            }}
            placeholder="Filter objects"
            spellCheck={false}
            className="min-w-0 flex-1 bg-transparent text-xs text-fg outline-none placeholder:text-faint"
          />
          {filter ? (
            <button
              type="button"
              aria-label="Clear filter"
              onClick={() => onFilterChange('')}
              className="grid shrink-0 place-items-center text-faint hover:text-fg"
            >
              <X size={12} />
            </button>
          ) : null}
        </div>

        <IconButton label="New connection" icon={<Plus size={14} />} onClick={onNewConnection} />
      </div>

      <div className="min-h-0 flex-1">
        {roots.length > 0 ? (
          <ObjectTree
            roots={roots}
            expanded={expanded}
            pickedIds={pickedIds}
            focusedId={focusedId}
            databaseFilters={databaseFilters}
            onToggle={onToggle}
            onSelect={onSelect}
            onActivate={onActivate}
            onDatabaseFilter={onDatabaseFilter}
            menuItemsFor={menuItemsFor}
          />
        ) : filter ? (
          <Empty
            icon={<SearchX size={22} strokeWidth={1.5} />}
            title={`Nothing matches “${filter}”`}
            hint="Only names already read are searched — a database nobody has opened has none yet."
          />
        ) : (
          <Empty
            icon={<Server size={22} strokeWidth={1.5} />}
            title="No connections yet"
            hint="Point one at a server and everything it holds appears here."
            action={
              <Button variant="primary" icon={<Plus size={14} />} onClick={onNewConnection}>
                New Connection
              </Button>
            }
          />
        )}
      </div>
    </aside>
  )
}
