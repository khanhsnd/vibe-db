import type { Cell } from '@/bridge/tables'

/**
 * The one separator a key space is read as a tree by. It is Redis's own convention rather than a
 * setting: `user:1:name` is how every server, library and client there is spells a namespace inside
 * a flat key space, and a key space is the only thing that has none of its own.
 */
export const KEY_SEPARATOR = ':'

/**
 * One row of the key tree, flattened. A **branch** is a prefix — it stands for everything under it —
 * and a **leaf** is one key. A prefix that is *also* a key of its own is both: the row carries that
 * key's own type, value and clock and still opens.
 *
 * The tree is flattened here rather than in the grid, for the reason `structureTree.ts` flattens
 * one: a grid has rows and not branches, so what it is given is a list, and the `depth` and the
 * caret on each row are what make it read as the tree it came from. See `GridOutline`.
 */
export interface KeyRow {
  depth: number
  /**
   * What this row is *called* — the whole key on a top-level leaf, and the tail segment below one,
   * the prefix being drawn on the row above it. It is not the key: `source` is what names that.
   */
  label: string
  /**
   * Which fetched row this stands for, and `-1` for a branch that is no key of its own. Nothing may
   * be written to such a row: there is no key there to write to.
   */
  source: number
  /** The prefix a fold is remembered under. A leaf has none, having nothing to fold. */
  branch: string | undefined
  /**
   * Open, shut, or `undefined` for a leaf — which is what the caret is drawn from, and what tells a
   * **key** from a **prefix**: the panel under the grid answers for a leaf and never for a branch,
   * a branch being a folder the reader is walking through rather than a key they picked.
   */
  open: boolean | undefined
  /** How many keys sit under this row, drawn as the pill beside its name. */
  count: number
}

/**
 * Every key of the page as one list of rows, grouped by prefix. The keys arrive **sorted** — a page
 * is cut from a sorted scan, which is the only way a page of a key space is the same page twice — so
 * every prefix is contiguous and the tree of a page is a coherent slice of the tree of the whole.
 *
 * A row whose key is not there yet is a **draft** the reader has just added, and it goes at the end
 * as a leaf of its own: it groups under nothing until it has been given a name.
 */
export function keyRows(keys: readonly Cell[], shut: ReadonlySet<string>): KeyRow[] {
  const root = emptyNode('', '')
  const drafts: KeyRow[] = []

  keys.forEach((key, source) => {
    if (key === null || key === '') {
      drafts.push({ depth: 0, label: '', source, branch: undefined, open: undefined, count: 0 })
      return
    }
    insert(root, key.split(KEY_SEPARATOR), source)
  })

  const rows: KeyRow[] = []
  emit(root, 0, shut, rows)
  return [...rows, ...drafts]
}

/**
 * How many keys a branch holds, for the grid to draw as a **pill beside** the name — see
 * `GridOutline.badge`. It is deliberately not spelled into the cell: a count inside the text is one a
 * copy writes out, a fit measures and a rename hands back as part of the name.
 */
export function keyBadge(row: KeyRow): string | undefined {
  return row.count > 0 ? String(row.count) : undefined
}

interface Node {
  label: string
  /** The whole prefix down to here, which is what a fold is remembered under. */
  path: string
  children: Map<string, Node>
  source: number
  /** How many keys sit *strictly* under this node. */
  count: number
}

function emptyNode(label: string, path: string): Node {
  return { label, path, children: new Map(), source: -1, count: 0 }
}

function insert(root: Node, segments: string[], source: number) {
  let node = root

  segments.forEach((segment, level) => {
    const path = node.path === '' ? segment : `${node.path}${KEY_SEPARATOR}${segment}`

    let child = node.children.get(segment)
    if (child === undefined) {
      child = emptyNode(segment, path)
      node.children.set(segment, child)
    }

    // Passing *through* a node is what makes it a prefix of this key, so counting here counts
    // exactly the keys under it — and the last segment is the key itself, not one under it.
    if (level === segments.length - 1) child.source = source
    else child.count += 1

    node = child
  })
}

/**
 * The tree as the list the grid reads, depth first. A `Map` keeps the order things were put into it,
 * and the keys went in sorted, so this comes out sorted at every level without a second sort.
 */
function emit(node: Node, depth: number, shut: ReadonlySet<string>, out: KeyRow[]) {
  for (const child of node.children.values()) {
    const branches = child.children.size > 0
    const open = branches ? !shut.has(child.path) : undefined

    out.push({
      depth,
      label: child.label,
      source: child.source,
      branch: branches ? child.path : undefined,
      open,
      count: child.count,
    })

    if (open === true) emit(child, depth + 1, shut, out)
  }
}
