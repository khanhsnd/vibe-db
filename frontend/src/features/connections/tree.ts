export type NodeKind =
  | 'connection'
  | 'database'
  | 'folder'
  | 'table'
  | 'view'
  | 'procedure'
  | 'function'

export type ConnectionStatus = 'idle' | 'connecting' | 'connected' | 'error'

export interface TreeNode {
  id: string
  kind: NodeKind
  label: string
  profileId: string
  database?: string
  children?: TreeNode[]
  status?: ConnectionStatus
  active?: boolean
  badge?: string
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
