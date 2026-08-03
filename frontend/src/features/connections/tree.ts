import type { Engine, Namespace } from '@/bridge/connections'

export type NodeKind =
  | 'connection'
  | 'database'
  | 'schema'
  | 'folder'
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
  status?: ConnectionStatus
  active?: boolean
  badge?: string
  /** A field that is part of the primary key, which the tree marks with a key rather than an icon. */
  primaryKey?: boolean
}

export interface FlatNode {
  node: TreeNode
  depth: number
  expanded: boolean
  expandable: boolean
}

export function isExpandable(node: TreeNode): boolean {
  return node.children !== undefined
}

/**
 * Whether this node is the one that holds the object folders, and so the one whose namespace
 * addresses tables. A MySQL database is that node; a PostgreSQL database is not, because its
 * children are schemas and they hold the folders.
 */
export function ownsObjects(node: TreeNode): boolean {
  return node.children?.[0]?.kind === 'folder'
}

export function flatten(roots: TreeNode[], expanded: ReadonlySet<string>): FlatNode[] {
  const visible: FlatNode[] = []

  const walk = (nodes: TreeNode[], depth: number) => {
    for (const node of nodes) {
      const expandable = isExpandable(node)
      const isOpen = expandable && expanded.has(node.id)
      visible.push({ node, depth, expanded: isOpen, expandable })
      if (isOpen && node.children?.length) walk(node.children, depth + 1)
    }
  }

  walk(roots, 0)
  return visible
}
