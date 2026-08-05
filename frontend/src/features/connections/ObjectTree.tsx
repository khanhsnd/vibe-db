import { useMemo, useRef } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import {
  ArrowDownAZ,
  Boxes,
  ChevronDown,
  ChevronRight,
  Columns3,
  Database,
  Eye,
  Folder,
  FolderOpen,
  KeyRound,
  Link,
  Server,
  Sigma,
  Table2,
  Terminal,
} from 'lucide-react'

import { cn } from '@/lib/cn'
import { TREE_ROW_HEIGHT } from '@/shared/theme/density'
import { ContextMenu, type MenuItem } from '@/shared/ui/ContextMenu'

import { EngineIcon } from './EngineIcon'
import { EnvironmentStripe } from './EnvironmentDot'
import { StatusDot } from './StatusDot'
import { flatten, isFolder, type NodeKind, type TreeNode } from './tree'

/**
 * A folder is drawn as a folder, open or shut, and never as the thing inside it: a `Tables` folder
 * wearing a table would be the same row as the tables under it. What it holds is said in its
 * colour instead — see `COLOURS`.
 */
const ICONS: Record<NodeKind, typeof Table2> = {
  connection: Server,
  database: Database,
  schema: Boxes,
  tables: Folder,
  views: Folder,
  routines: Folder,
  table: Table2,
  view: Eye,
  procedure: Terminal,
  function: Sigma,
  fields: Folder,
  field: Columns3,
  indexes: Folder,
  index: ArrowDownAZ,
  foreignKeys: Folder,
  foreignKey: Link,
}

/**
 * The colour each row is drawn in. A folder takes the colour of its contents rather than one of
 * its own, so `Indexes` and the indexes under it read as one thing — six levels of grey icons say
 * only that there are six levels.
 */
const COLOURS: Record<NodeKind, string> = {
  connection: 'text-muted',
  database: 'text-kind-database',
  schema: 'text-kind-schema',
  tables: 'text-kind-table',
  views: 'text-kind-view',
  routines: 'text-kind-routine',
  table: 'text-kind-table',
  view: 'text-kind-view',
  procedure: 'text-kind-routine',
  function: 'text-kind-routine',
  fields: 'text-kind-field',
  field: 'text-kind-field',
  indexes: 'text-kind-index',
  index: 'text-kind-index',
  foreignKeys: 'text-kind-relation',
  foreignKey: 'text-kind-relation',
}

const INDENT_PER_DEPTH = 12
const BASE_INDENT = 6

/**
 * Whether a badge is how many rather than what: every count the tree draws is a bare number, and
 * nothing else it draws is — a type carries letters, a key's target an arrow, a server a colon.
 */
const isCount = (badge: string) => /^\d+$/.test(badge)

interface ObjectTreeProps {
  roots: TreeNode[]
  expanded: ReadonlySet<string>
  selectedId: string | null
  onToggle: (node: TreeNode) => void
  onSelect: (node: TreeNode) => void
  onActivate: (node: TreeNode) => void
  menuItemsFor?: (node: TreeNode) => MenuItem[]
}

export function ObjectTree({
  roots,
  expanded,
  selectedId,
  onToggle,
  onSelect,
  onActivate,
  menuItemsFor,
}: ObjectTreeProps) {
  const scrollRef = useRef<HTMLDivElement>(null)
  const rows = useMemo(() => flatten(roots, expanded), [roots, expanded])

  const virtualizer = useVirtualizer({
    count: rows.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => TREE_ROW_HEIGHT,
    overscan: 12,
  })

  return (
    <div ref={scrollRef} className="scroll-thin h-full overflow-auto" role="tree">
      <div className="relative w-full" style={{ height: virtualizer.getTotalSize() }}>
        {virtualizer.getVirtualItems().map((item) => {
          const row = rows[item.index]
          if (!row) return null

          const { node, depth, expanded: isOpen, expandable } = row
          const folder = isFolder(node)
          // A field of the primary key is drawn with a key rather than a column, which is the
          // one place the icon says something the label does not. A folder is drawn open once it
          // is, which is the one place the icon says what the caret beside it already said —
          // and it is worth the repetition, a caret being three pixels of chevron.
          const Icon =
            node.primaryKey === true
              ? KeyRound
              : folder && isOpen
                ? FolderOpen
                : ICONS[node.kind]
          const selected = node.id === selectedId
          const active = node.active === true

          return (
            <ContextMenu key={node.id} items={menuItemsFor?.(node) ?? []}>
              <div
                role="treeitem"
                aria-level={depth + 1}
                aria-selected={selected}
                aria-expanded={expandable ? isOpen : undefined}
                onClick={() => onSelect(node)}
                onContextMenu={() => onSelect(node)}
                onDoubleClick={() => onActivate(node)}
                // Indenting six levels leaves a field's name little room, so what a row could
                // not draw is still readable by hovering it.
                title={node.badge ? `${node.label} — ${node.badge}` : node.label}
                className={cn(
                  'absolute top-0 left-0 flex w-full items-center gap-1 pr-1.5',
                  'cursor-default text-xs whitespace-nowrap',
                  selected ? 'bg-selected text-fg' : 'text-fg hover:bg-hover',
                )}
                style={{
                  height: TREE_ROW_HEIGHT,
                  transform: `translateY(${item.start}px)`,
                  paddingLeft: BASE_INDENT + depth * INDENT_PER_DEPTH,
                }}
              >
                {/* Only a connection carries one, and everything under it inherits the warning
                    by being under it — see EnvironmentStripe. */}
                <EnvironmentStripe environment={node.environment ?? ''} />

                {expandable ? (
                  <button
                    type="button"
                    aria-label={isOpen ? 'Collapse' : 'Expand'}
                    onClick={(e) => {
                      e.stopPropagation()
                      onSelect(node)
                      onToggle(node)
                    }}
                    onDoubleClick={(e) => e.stopPropagation()}
                    className="grid h-4 w-3.5 shrink-0 place-items-center text-muted hover:text-fg"
                  >
                    {isOpen ? <ChevronDown size={12} /> : <ChevronRight size={12} />}
                  </button>
                ) : (
                  <span className="w-3.5 shrink-0" />
                )}

                {node.engine ? (
                  <EngineIcon engine={node.engine} />
                ) : (
                  <Icon
                    size={13}
                    className={cn(
                      'shrink-0',
                      node.primaryKey === true
                        ? 'text-grid-key'
                        : active
                          ? 'text-success'
                          : COLOURS[node.kind],
                    )}
                  />
                )}

                <span
                  className={cn(
                    'min-w-0 flex-1 truncate',
                    active && 'font-medium text-success',
                    selected && 'font-medium',
                  )}
                >
                  {node.label}
                </span>

                {/* A count is a pill and a fact is not: `12` beside `Tables` means twelve of
                    them, where `varchar(255)` beside a field is what that field *is*, and the
                    two would read alike drawn alike. */}
                {node.badge ? (
                  isCount(node.badge) ? (
                    <span className="shrink-0 rounded-full bg-active px-1.5 font-mono text-2xs text-muted tabular-nums">
                      {node.badge}
                    </span>
                  ) : (
                    <span className="max-w-[45%] shrink-0 truncate pl-1.5 font-mono text-2xs text-faint">
                      {node.badge}
                    </span>
                  )
                ) : null}

                {node.status ? <StatusDot status={node.status} /> : null}
              </div>
            </ContextMenu>
          )
        })}
      </div>
    </div>
  )
}
