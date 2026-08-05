import type { Engine, Namespace, Routine } from '@/bridge/connections'

/**
 * A folder is named after what it holds — `tables`, `views`, `routines` — the way a table's own
 * folders are, so a right-click knows what it is over without a second field to say so.
 */
export type NodeKind =
  | 'connection'
  | 'database'
  | 'schema'
  | 'tables'
  | 'views'
  | 'routines'
  | 'table'
  | 'view'
  | 'procedure'
  | 'function'
  | 'fields'
  | 'field'
  | 'indexes'
  | 'index'
  | 'foreignKeys'
  | 'foreignKey'

export type ConnectionStatus = 'idle' | 'connecting' | 'connected' | 'error'

export interface TreeNode {
  id: string
  kind: NodeKind
  label: string
  profileId: string
  /** Where this node's tables live. Absent on a connection, which has no namespace yet. */
  namespace?: Namespace
  /** The table this node belongs to, on a table node and everything the tree draws under one. */
  table?: string
  children?: TreeNode[]
  /** The engine this connection speaks, which is drawn as its chip. Only a connection has one. */
  engine?: Engine
  /**
   * What this connection is *for*, drawn as the stripe down its row. Only a connection has one:
   * everything under it belongs to that connection and inherits the warning by being under it.
   */
  environment?: string
  /**
   * The colour the reader marked this connection with. Only a connection has one, and everything
   * under it is washed with it by being under it — see `flatten`.
   */
  colour?: string
  status?: ConnectionStatus
  active?: boolean
  badge?: string
  /** A field that is part of the primary key, which the tree marks with a key rather than an icon. */
  primaryKey?: boolean
  /** The routine a `procedure` or `function` row stands for, which is what a drop is addressed to. */
  routine?: Routine
}

export interface FlatNode {
  node: TreeNode
  depth: number
  expanded: boolean
  expandable: boolean
  /** The colour of the connection this row belongs to, inherited from the root of its branch. */
  colour: string
}

export function isExpandable(node: TreeNode): boolean {
  return node.children !== undefined
}

const OBJECT_FOLDERS: ReadonlySet<NodeKind> = new Set(['tables', 'views', 'routines'])

/**
 * Whether this node is one of a namespace's object folders. All three are filled by one read,
 * so opening any of them asks for the same thing.
 */
export function isObjectFolder(node: TreeNode): boolean {
  return OBJECT_FOLDERS.has(node.kind)
}

/**
 * Whether this node is the one that holds the object folders, and so the one whose namespace
 * addresses tables. A MySQL database is that node; a PostgreSQL database is not, because its
 * children are schemas and they hold the folders. It is *any* folder and not the first one,
 * because a search hands back a namespace narrowed to whichever of its folders held a hit.
 */
export function ownsObjects(node: TreeNode): boolean {
  return node.children?.some(isObjectFolder) ?? false
}

/** Whether this node's rows can be opened in a data tab: a table, or a view, which reads alike. */
export function hasRows(node: TreeNode): boolean {
  return node.kind === 'table' || node.kind === 'view'
}

const FOLDERS: ReadonlySet<NodeKind> = new Set([
  'tables',
  'views',
  'routines',
  'fields',
  'indexes',
  'foreignKeys',
])

/**
 * Whether this node holds others rather than standing for something in the catalogue. It is what
 * decides how a badge reads: a folder's is how many are inside, and a leaf's is a fact about the
 * thing itself — a field's type, an index's columns, the table a key points at.
 */
export function isFolder(node: TreeNode): boolean {
  return FOLDERS.has(node.kind)
}

export interface TreeSearch {
  roots: TreeNode[]
  /** The folders above a hit, which are opened whatever the reader had left open. */
  expanded: ReadonlySet<string>
}

/**
 * The tree narrowed to the rows whose name holds `query`, and `null` when nothing was typed. A
 * folder never matches — `Tables` sits under every namespace, so matching it would say nothing —
 * and a hit keeps its own children shut, the reader having asked for that row rather than for
 * everything under it. Only what has been read is searched: the tree fills a level at a time, so a
 * namespace nobody has opened holds no names to find.
 */
export function searchTree(roots: TreeNode[], query: string): TreeSearch | null {
  const needle = query.trim().toLowerCase()
  if (needle === '') return null

  const expanded = new Set<string>()

  const keep = (node: TreeNode): TreeNode | null => {
    if (!FOLDERS.has(node.kind) && node.label.toLowerCase().includes(needle)) return node

    const children = (node.children ?? []).map(keep).filter((child) => child !== null)
    if (children.length === 0) return null

    expanded.add(node.id)
    return { ...node, children }
  }

  return { roots: roots.map(keep).filter((root) => root !== null), expanded }
}

export function flatten(roots: TreeNode[], expanded: ReadonlySet<string>): FlatNode[] {
  const visible: FlatNode[] = []

  // A colour is set on a connection and inherited by everything under it, which is the whole of
  // what "this connection is the blue one" means: a database of it is that connection's database.
  const walk = (nodes: TreeNode[], depth: number, colour: string) => {
    for (const node of nodes) {
      const expandable = isExpandable(node)
      const isOpen = expandable && expanded.has(node.id)
      const own = node.colour ?? colour
      visible.push({ node, depth, expanded: isOpen, expandable, colour: own })
      if (isOpen && node.children?.length) walk(node.children, depth + 1, own)
    }
  }

  walk(roots, 0, '')
  return visible
}
