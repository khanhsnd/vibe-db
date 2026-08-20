import { useMemo, useRef, type KeyboardEvent } from 'react'
import { useVirtualizer } from '@tanstack/react-virtual'
import {
  Activity,
  Archive,
  ArrowDownAZ,
  Boxes,
  CalendarClock,
  ChevronDown,
  ChevronRight,
  Cloud,
  Columns3,
  Database,
  Eye,
  Folder,
  FolderOpen,
  Hash,
  KeyRound,
  Layers,
  Link,
  Loader2,
  Server,
  Sigma,
  Split,
  Table2,
  Terminal,
  UserRound,
  Zap,
} from 'lucide-react'

import { cn } from '@/lib/cn'
import { TREE_ROW_HEIGHT } from '@/shared/theme/density'
import { ContextMenu, type MenuItem } from '@/shared/ui/ContextMenu'

import { StoreIcon } from '@/features/storage/StoreIcon'

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
  // A store connection wears its provider's chip rather than this, exactly as a database connection
  // wears its engine's — the glyph is what a row falls back to while nothing is known about it.
  store: Cloud,
  // A bucket is drawn as the pail it is, and never as a folder: a folder in this tree is a heading
  // the app made up, and a bucket is a thing the service holds.
  bucket: Archive,
  database: Database,
  schema: Boxes,
  tables: Folder,
  views: Folder,
  routines: Folder,
  triggers: Folder,
  // Drawn as a folder, being one, and its files as the archives they are — the one branch of the
  // tree whose leaves are on this machine rather than on the server.
  backups: Folder,
  backup: Archive,
  // An account is drawn as a person because that is what it is — the one row in the tree that
  // stands for somebody rather than for something the server stores.
  accounts: Folder,
  account: UserRound,
  // What the server is doing, drawn as the pulse it is: the one row in the tree whose contents change
  // while nobody touches anything.
  monitor: Activity,
  materializedViews: Folder,
  sequences: Folder,
  events: Folder,
  table: Table2,
  view: Eye,
  procedure: Terminal,
  function: Sigma,
  trigger: Zap,
  // A materialized view is drawn as the layers it is: a view with the rows it read kept underneath.
  materializedView: Layers,
  // A sequence hands out numbers, and a clock is what an event runs on.
  sequence: Hash,
  event: CalendarClock,
  fields: Folder,
  field: Columns3,
  indexes: Folder,
  index: ArrowDownAZ,
  foreignKeys: Folder,
  foreignKey: Link,
  partitions: Folder,
  partition: Split,
}

/**
 * The colour each row is drawn in. A folder takes the colour of its contents rather than one of
 * its own, so `Indexes` and the indexes under it read as one thing — six levels of grey icons say
 * only that there are six levels.
 */
const COLOURS: Record<NodeKind, string> = {
  connection: 'text-muted',
  store: 'text-muted',
  bucket: 'text-kind-bucket',
  database: 'text-kind-database',
  schema: 'text-kind-schema',
  tables: 'text-kind-table',
  views: 'text-kind-view',
  routines: 'text-kind-routine',
  triggers: 'text-kind-trigger',
  backups: 'text-kind-backup',
  backup: 'text-kind-backup',
  accounts: 'text-kind-account',
  account: 'text-kind-account',
  // A process is a statement in flight, so it wears the colour a *program* wears everywhere in this
  // app rather than taking an entry of its own — the same colour a query tab has in the strip.
  monitor: 'text-kind-routine',
  materializedViews: 'text-kind-view',
  sequences: 'text-kind-sequence',
  // An event is a program the server runs, so it wears the colour every other program here wears.
  events: 'text-kind-routine',
  table: 'text-kind-table',
  view: 'text-kind-view',
  procedure: 'text-kind-routine',
  function: 'text-kind-routine',
  trigger: 'text-kind-trigger',
  // A materialized view *is* a view, so it reads as one; what makes it different is what can be
  // done to it, which is the menu's business and not the colour's.
  materializedView: 'text-kind-view',
  sequence: 'text-kind-sequence',
  event: 'text-kind-routine',
  fields: 'text-kind-field',
  field: 'text-kind-field',
  indexes: 'text-kind-index',
  index: 'text-kind-index',
  foreignKeys: 'text-kind-relation',
  foreignKey: 'text-kind-relation',
  partitions: 'text-kind-partition',
  partition: 'text-kind-partition',
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

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== 'F2' || focusedId === null) return
    if (event.target instanceof HTMLButtonElement || event.target instanceof HTMLInputElement) return

    const node = rows.find((row) => row.node.id === focusedId)?.node
    const rename = node ? menuItemsFor?.(node).find((item) => item.shortcut === 'F2') : undefined
    if (!rename?.onSelect || rename.disabled) return

    event.preventDefault()
    rename.onSelect()
  }

  return (
    <div
      ref={scrollRef}
      tabIndex={0}
      onKeyDown={onKeyDown}
      className="scroll-thin h-full overflow-auto"
      role="tree"
    >
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
                tabIndex={focused ? 0 : -1}
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

                {node.provider !== undefined ? (
                  // A store connection is dialled too — a bastion in front of a private MinIO takes
                  // exactly as long as one in front of a database — so it gets the same spinner in
                  // the same place rather than a dot on the far right nobody watches.
                  node.status === 'connecting' ? (
                    <span className="grid h-4 w-4 shrink-0 place-items-center" title="Connecting…">
                      <Loader2 size={13} className="animate-spin text-accent" />
                    </span>
                  ) : (
                    <StoreIcon provider={node.provider} />
                  )
                ) : node.engine ? (
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
