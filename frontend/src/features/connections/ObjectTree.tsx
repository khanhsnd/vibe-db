import { useMemo, useRef } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import {
  ChevronDown,
  ChevronRight,
  Database,
  Eye,
  Folder,
  Server,
  Sigma,
  Table2,
  Terminal,
} from 'lucide-react'

import { cn } from '@/lib/cn'
import { ROW_HEIGHT } from '@/shared/theme/density'
import { ContextMenu, type MenuItem } from '@/shared/ui/ContextMenu'

import { StatusDot } from './StatusDot'
import { flatten, type NodeKind, type TreeNode } from './tree'

const ICONS: Record<NodeKind, typeof Table2> = {
  connection: Server,
  database: Database,
  folder: Folder,
  table: Table2,
  view: Eye,
  procedure: Terminal,
  function: Sigma,
}

const INDENT_PER_DEPTH = 14
const BASE_INDENT = 6

interface ObjectTreeProps {
  roots: TreeNode[]
  expanded: ReadonlySet<string>
  selectedId: string | null
  onToggle: (id: string) => void
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
    estimateSize: () => ROW_HEIGHT,
    overscan: 12,
  })

  return (
    <div ref={scrollRef} className="scroll-thin h-full overflow-auto" role="tree">
      <div className="relative w-full" style={{ height: virtualizer.getTotalSize() }}>
        {virtualizer.getVirtualItems().map((item) => {
          const row = rows[item.index]
          if (!row) return null

          const { node, depth, expanded: isOpen, expandable } = row
          const Icon = ICONS[node.kind]
          const selected = node.id === selectedId

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
                className={cn(
                  'absolute top-0 left-0 flex w-full items-center gap-1.5 pr-2',
                  'cursor-default text-[12.5px] whitespace-nowrap',
                  selected ? 'bg-selected text-fg' : 'text-muted hover:bg-hover hover:text-fg',
                )}
                style={{
                  height: ROW_HEIGHT,
                  transform: `translateY(${item.start}px)`,
                  paddingLeft: BASE_INDENT + depth * INDENT_PER_DEPTH,
                }}
              >
                {expandable ? (
                  <button
                    type="button"
                    aria-label={isOpen ? 'Collapse' : 'Expand'}
                    onClick={(e) => {
                      e.stopPropagation()
                      onToggle(node.id)
                    }}
                    className="grid h-4 w-4 shrink-0 place-items-center text-faint hover:text-fg"
                  >
                    {isOpen ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
                  </button>
                ) : (
                  <span className="w-4 shrink-0" />
                )}

                {node.kind === 'connection' && node.status ? (
                  <StatusDot status={node.status} />
                ) : (
                  <Icon size={13} className="shrink-0 text-faint" />
                )}

                <span className="truncate">{node.label}</span>

                {node.badge ? (
                  <span className="ml-auto font-mono text-[11px] text-faint">{node.badge}</span>
                ) : null}
              </div>
            </ContextMenu>
          )
        })}
      </div>
    </div>
  )
}
