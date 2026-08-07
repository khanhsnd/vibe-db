import { useMemo, useRef } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import {
  Archive,
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
  Loader2,
  Server,
  Sigma,
  Table2,
  Terminal,
} from 'lucide-react'

import { cn } from '@/lib/cn'
import { TREE_ROW_HEIGHT } from '@/shared/theme/density'
import { ContextMenu, type MenuItem } from '@/shared/ui/ContextMenu'

import { colourTint } from './ConnectionColour'
import { DatabaseFilterButton } from './DatabaseFilterPanel'
import { EngineIcon } from './EngineIcon'
import { EnvironmentStripe } from './EnvironmentDot'
import { StatusDot } from './StatusDot'
import { flatten, isFolder, type NodeKind, type PickHow, type TreeNode } from './tree'

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
  // Drawn as a folder, being one, and its files as the archives they are — the one branch of the
  // tree whose leaves are on this machine rather than on the server.
  backups: Folder,
  backup: Archive,
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
  backups: 'text-kind-backup',
  backup: 'text-kind-backup',
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
 * The glyph a kind of catalogue object is drawn as, in its own colour — the tree's, and anywhere
 * else one is drawn. A field means the same thing in the sidebar and in a comparison, so it wears
 * the same shape and the same `--kind-*` colour in both; a second map somewhere else would drift
 * the day after it was written.
 */
export function nodeGlyph(kind: NodeKind, open = false, size = 13) {
  const Icon = isFolder({ kind } as TreeNode) && open ? FolderOpen : ICONS[kind]
  return <Icon size={size} className={cn('shrink-0', COLOURS[kind])} />
}

/**
 * Whether a badge is how many rather than what: every count the tree draws is a bare number, and
 * nothing else it draws is — a type carries letters, a key's target an arrow, a server a colon.
 */
const isCount = (badge: string) => /^\d+$/.test(badge)

interface ObjectTreeProps {
  roots: TreeNode[]
  expanded: ReadonlySet<string>
  /**
   * Every row the reader has picked, and the **focused** one — the last press to land. Two depths
   * rather than one, exactly as the grid has: a picked row is washed and the focused one is washed
   * and bold, so a pair picked with `Ctrl` reads as a pair and the reader still knows where they
   * are. A single press leaves the two the same row, which is every press but a `Ctrl` one.
   */
  pickedIds: ReadonlySet<string>
  focusedId: string | null
  /** What each connection draws of its databases, keyed by connection — see `databaseFilter.ts`. */
  databaseFilters: Record<string, string>
  onToggle: (node: TreeNode) => void
  onSelect: (node: TreeNode, how: PickHow) => void
  onActivate: (node: TreeNode) => void
  onDatabaseFilter: (profileId: string, pattern: string) => void
  menuItemsFor?: (node: TreeNode) => MenuItem[]
}

export function ObjectTree({
  roots,
  expanded,
  pickedIds,
  focusedId,
  databaseFilters,
  onToggle,
  onSelect,
  onActivate,
  onDatabaseFilter,
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

          const { node, depth, expanded: isOpen, expandable, colour } = row
          const folder = isFolder(node)
          const tint = colourTint(colour)
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
          const picked = pickedIds.has(node.id)
          const focused = node.id === focusedId
          const active = node.active === true

          return (
            <ContextMenu key={node.id} items={menuItemsFor?.(node) ?? []}>
              <div
                role="treeitem"
                aria-level={depth + 1}
                aria-selected={picked}
                aria-expanded={expandable ? isOpen : undefined}
                // `Ctrl`/`Cmd` picks a second row beside the first rather than replacing it,
                // which is how a pair is named for Compare — and is the same modifier that adds
                // a rectangle in the grid.
                onClick={(e) => onSelect(node, e.ctrlKey || e.metaKey ? 'add' : 'replace')}
                onContextMenu={() => onSelect(node, 'menu')}
                onDoubleClick={() => onActivate(node)}
                // Indenting six levels leaves a field's name little room, so what a row could
                // not draw is still readable by hovering it.
                title={node.badge ? `${node.label} — ${node.badge}` : node.label}
                className={cn(
                  'group absolute top-0 left-0 flex w-full items-center gap-1 pr-1.5',
                  'cursor-default text-xs whitespace-nowrap',
                  picked ? 'bg-selected text-fg' : 'text-fg hover:bg-hover',
                )}
                style={{
                  height: TREE_ROW_HEIGHT,
                  transform: `translateY(${item.start}px)`,
                  paddingLeft: BASE_INDENT + depth * INDENT_PER_DEPTH,
                }}
              >
                {/* The connection's own colour, washed over the row. It sits behind the row's
                    text and in front of its background, so hovering and selecting still read
                    through it rather than being painted out by it. */}
                {tint ? (
                  <span
                    aria-hidden
                    style={{ background: tint }}
                    className="pointer-events-none absolute inset-0 -z-10"
                  />
                ) : null}

                {/* Only a connection carries one, and everything under it inherits the warning
                    by being under it — see EnvironmentStripe. */}
                <EnvironmentStripe environment={node.environment ?? ''} />

                {expandable ? (
                  <button
                    type="button"
                    aria-label={isOpen ? 'Collapse' : 'Expand'}
                    onClick={(e) => {
                      e.stopPropagation()
                      onSelect(node, 'caret')
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
                  // A connection being dialled wears a spinner where its chip goes — the one place
                  // in the tree an icon says what is *happening* rather than what a row is. A dot
                  // on the far right is easy to miss on the row the reader has just pressed, and
                  // dialling a bastion and a server behind it takes long enough to be doubted.
                  node.status === 'connecting' ? (
                    <span className="grid h-4 w-4 shrink-0 place-items-center" title="Connecting…">
                      <Loader2 size={13} className="animate-spin text-accent" />
                    </span>
                  ) : (
                    <EngineIcon engine={node.engine} />
                  )
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
                    focused && 'font-medium',
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

                {/* Only a connection carries one: what it narrows is that connection's database
                    list, and every level below is under one database already. */}
                {node.kind === 'connection' ? (
                  <DatabaseFilterButton
                    pattern={databaseFilters[node.profileId] ?? ''}
                    // A shut connection has listed nothing, so it has no number rather than a zero:
                    // its branch is empty for a reason no pattern is responsible for.
                    shown={node.status === 'connected' ? (node.children?.length ?? 0) : null}
                    onChange={(typed) => onDatabaseFilter(node.profileId, typed)}
                  />
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
