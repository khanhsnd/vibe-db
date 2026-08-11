import type { TableColumn } from '@/bridge/tables'
import type { TableStructure } from '@/bridge/schema'

/**
 * What a diagram *is*, and every measurement anything drawing one takes.
 *
 * A diagram holds **no shape of its own**: a box names a table and nothing more, and the fields
 * inside it, the keys marked on them and the lines between them are read from the catalogue the tree
 * already loaded. So a column added by somebody else appears on a Refresh rather than leaving the
 * picture quietly wrong — which is the whole difference between this and an offline modelling tool.
 *
 * What is *the reader's* is where a box sits, how wide it is, what colour it wears, and the notes
 * and shapes drawn beside it. That, and only that, is what `persist.ts` writes down.
 */

/** What a press on the canvas does. Every one of them is a mode the toolbar names. */
export type Tool = 'select' | 'pan' | 'note' | 'rectangle' | 'ellipse' | 'text' | 'relation'

/** The four things a reader can draw beside the tables. A `text` has no box, only its words. */
export type ShapeKind = 'note' | 'rectangle' | 'ellipse' | 'text'

/**
 * How much of a table a box draws, for the whole diagram at once. A schema of sixty tables is read
 * as a shape rather than as sixty field lists, so `keys` — the columns a relation is actually drawn
 * to — and `title` are what make one legible; `full` is what makes one worth reading closely.
 */
export type DisplayMode = 'full' | 'keys' | 'title'

export const DISPLAY_MODES: ReadonlyArray<{ value: DisplayMode; label: string }> = [
  { value: 'full', label: 'All fields' },
  { value: 'keys', label: 'Keys only' },
  { value: 'title', label: 'Table names only' },
]

/** One table on the canvas. It is named by the table, so a table cannot be on a diagram twice. */
export interface DiagramNode {
  table: string
  x: number
  y: number
  width: number
  /** Drawn as its header alone, whatever the diagram's display mode says. */
  collapsed: boolean
  /** One of `CONNECTION_COLOURS`, or `''` for the box's own shade. */
  colour: string
}

/** One thing the reader drew. It belongs to nobody on the server and means whatever they wrote in it. */
export interface DiagramShape {
  id: string
  kind: ShapeKind
  x: number
  y: number
  width: number
  height: number
  text: string
  colour: string
}

export interface Diagram {
  nodes: DiagramNode[]
  shapes: DiagramShape[]
  display: DisplayMode
  /** Whether a drag lands on the grid, and whether that grid is drawn. */
  snap: boolean
  grid: boolean
  /** Whether each line carries the constraint's own name. Off by default: a schema of forty keys
      reads as forty names before it reads as a shape. */
  names: boolean
  zoom: number
  panX: number
  panY: number
}

/** A diagram nobody has drawn yet, and a referentially stable default for one not started. */
export const EMPTY_DIAGRAM: Diagram = {
  nodes: [],
  shapes: [],
  display: 'full',
  snap: true,
  grid: true,
  names: false,
  zoom: 1,
  panX: 40,
  panY: 40,
}

export const HEADER_HEIGHT = 24
export const ROW_HEIGHT = 18
export const DEFAULT_NODE_WIDTH = 210
export const MIN_NODE_WIDTH = 130
export const GRID_STEP = 10
/** How far a line stands off a box before it turns, and the radius its corners are rounded by. */
export const STUB = 18
const CORNER = 6

export const ZOOM_MIN = 0.2
export const ZOOM_MAX = 3

export function clampZoom(zoom: number): number {
  return Math.min(ZOOM_MAX, Math.max(ZOOM_MIN, zoom))
}

export function snapped(value: number, snap: boolean): number {
  return snap ? Math.round(value / GRID_STEP) * GRID_STEP : Math.round(value)
}

/** What the reader has picked, as one string per thing, so the selection is a plain stable list. */
export type SelectionKind = 'node' | 'shape' | 'relation'

export const pickKey = (kind: SelectionKind, id: string) => `${kind}:${id}`
export const nodeKey = (table: string) => pickKey('node', table)
export const shapeKey = (id: string) => pickKey('shape', id)
export const relationKey = (id: string) => pickKey('relation', id)

export function pickedOf(selection: readonly string[], kind: SelectionKind): string[] {
  const prefix = `${kind}:`
  return selection.flatMap((key) => (key.startsWith(prefix) ? [key.slice(prefix.length)] : []))
}

/**
 * One line on the canvas: a foreign key held by `from` and pointing at `to`. Its id is the table
 * that holds the constraint and the constraint's own name, because two tables may each hold a
 * constraint of the same name and only the pair names one of them.
 */
export interface Relation {
  id: string
  name: string
  /** The table holding the key — the *many* end, which is where the crow's foot is drawn. */
  from: string
  fromColumns: string[]
  to: string
  toColumns: string[]
  onUpdate: string
  onDelete: string
  /** Whether every column of the key may be null, which is what makes the end optional. */
  optional: boolean
}

export const relationId = (table: string, constraint: string) => `${table} ${constraint}`

/**
 * A line the reader has drawn and no server has heard of yet — what the foreign key dialog opens on.
 * A blank column is a drop that landed on a box rather than on one of its rows.
 */
export interface LinkDraft {
  from: string
  fromColumn: string
  to: string
  toColumn: string
}

/** A box as it is drawn: where it is, how big, and which fields are inside it. */
export interface Box {
  node: DiagramNode
  fields: TableColumn[]
  /** Whether the structure has been read at all — an unread table draws its header and a hint. */
  known: boolean
  width: number
  height: number
  /** How many fields the table has, whatever the display mode draws of them. */
  total: number
  /** The keys marked on the drawn fields, so a row is drawn without asking the structure again. */
  primary: ReadonlySet<string>
  foreign: ReadonlySet<string>
}

/** Which columns a box draws, which is the display mode's answer and the node's own collapse. */
export function drawnFields(
  structure: TableStructure | undefined,
  node: DiagramNode,
  display: DisplayMode,
): TableColumn[] {
  if (!structure || node.collapsed || display === 'title') return []
  if (display === 'full') return structure.columns

  const keyed = new Set<string>()
  for (const column of structure.columns) {
    if (column.primaryKey) keyed.add(column.name)
  }
  for (const key of structure.foreignKeys) {
    for (const column of key.columns) keyed.add(column)
  }
  return structure.columns.filter((column) => keyed.has(column.name))
}

export function boxOf(
  node: DiagramNode,
  structure: TableStructure | undefined,
  display: DisplayMode,
): Box {
  const fields = drawnFields(structure, node, display)
  const foreign = new Set<string>()
  for (const key of structure?.foreignKeys ?? []) {
    for (const column of key.columns) foreign.add(column)
  }

  return {
    node,
    fields,
    known: structure !== undefined,
    width: Math.max(MIN_NODE_WIDTH, node.width),
    height: HEADER_HEIGHT + fields.length * ROW_HEIGHT,
    total: structure?.columns.length ?? 0,
    primary: new Set(structure?.columns.filter((one) => one.primaryKey).map((one) => one.name)),
    foreign,
  }
}

/** Every relation between two tables **both on the canvas**; a key pointing off it is not drawn. */
export function relationsOf(
  nodes: readonly DiagramNode[],
  structures: Record<string, TableStructure | undefined>,
  keyOf: (table: string) => string,
): Relation[] {
  const drawn = new Set(nodes.map((node) => node.table))

  return nodes.flatMap((node) => {
    const structure = structures[keyOf(node.table)]
    if (!structure) return []

    const nullable = new Set(
      structure.columns.filter((column) => column.nullable).map((column) => column.name),
    )

    return structure.foreignKeys.flatMap((key) => {
      if (!drawn.has(key.referencedTable)) return []

      return [
        {
          id: relationId(node.table, key.name),
          name: key.name,
          from: node.table,
          fromColumns: key.columns,
          to: key.referencedTable,
          toColumns: key.referencedColumns,
          onUpdate: key.onUpdate,
          onDelete: key.onDelete,
          optional: key.columns.every((column) => nullable.has(column)),
        },
      ]
    })
  })
}

/** Where a line meets a box: the point, and which way it leaves — `1` right, `-1` left. */
export interface Anchor {
  x: number
  y: number
  dir: 1 | -1
}

/**
 * Where on a box a relation's end sits. It is the **row of the column** where that column is drawn,
 * and the header where it is not — a collapsed box, or one whose display mode leaves the column
 * out — so a line always lands on something the reader can see.
 */
export function anchorOf(box: Box, column: string | undefined, dir: 1 | -1): Anchor {
  const at = column === undefined ? -1 : box.fields.findIndex((one) => one.name === column)
  const y =
    at < 0
      ? box.node.y + HEADER_HEIGHT / 2
      : box.node.y + HEADER_HEIGHT + at * ROW_HEIGHT + ROW_HEIGHT / 2

  return { x: dir > 0 ? box.node.x + box.width : box.node.x, y, dir }
}

/** Which sides of two boxes face each other, which is what keeps a line out of the boxes it joins. */
export function facing(from: Box, to: Box): { from: 1 | -1; to: 1 | -1 } {
  const fromRight = from.node.x + from.width
  const toRight = to.node.x + to.width

  if (fromRight + STUB * 2 <= to.node.x) return { from: 1, to: -1 }
  if (toRight + STUB * 2 <= from.node.x) return { from: -1, to: 1 }

  // The two overlap, so both ends leave the same way and the line goes round: whichever side has
  // the box edges nearer together is the side that costs the least ink.
  const left = Math.min(from.node.x, to.node.x)
  const right = Math.max(fromRight, toRight)
  const middle = (from.node.x + fromRight + to.node.x + toRight) / 4
  return middle - left < right - middle ? { from: -1, to: -1 } : { from: 1, to: 1 }
}

export interface Point {
  x: number
  y: number
}

/** The corners a line turns at, from one anchor to the other. */
export function elbow(from: Anchor, to: Anchor): Point[] {
  const out = { x: from.x + from.dir * STUB, y: from.y }
  const back = { x: to.x + to.dir * STUB, y: to.y }

  if (from.dir === to.dir) {
    const side = from.dir > 0 ? Math.max(out.x, back.x) : Math.min(out.x, back.x)
    return [
      { x: from.x, y: from.y },
      { x: side, y: from.y },
      { x: side, y: to.y },
      { x: to.x, y: to.y },
    ]
  }

  const middle = (out.x + back.x) / 2
  return [
    { x: from.x, y: from.y },
    { x: middle, y: from.y },
    { x: middle, y: to.y },
    { x: to.x, y: to.y },
  ]
}

/** A polyline with its corners rounded, which is the one path shape anything here draws. */
export function polyline(points: readonly Point[]): string {
  const [first, ...rest] = points
  if (!first) return ''
  if (rest.length === 0) return `M ${first.x} ${first.y}`

  let path = `M ${first.x} ${first.y}`
  for (let at = 1; at < points.length; at += 1) {
    const previous = points[at - 1]
    const corner = points[at]
    const next = points[at + 1]
    if (!previous || !corner) continue

    if (!next) {
      path += ` L ${corner.x} ${corner.y}`
      continue
    }

    const before = shorten(corner, previous)
    const after = shorten(corner, next)
    path += ` L ${before.x} ${before.y} Q ${corner.x} ${corner.y} ${after.x} ${after.y}`
  }
  return path
}

// The point `CORNER` along the way from a corner towards its neighbour — or the midpoint, where the
// two are nearer than that, so a short segment never turns the curve inside out.
function shorten(corner: Point, towards: Point): Point {
  const dx = towards.x - corner.x
  const dy = towards.y - corner.y
  const length = Math.hypot(dx, dy)
  if (length === 0) return corner

  const step = Math.min(CORNER, length / 2)
  return { x: corner.x + (dx / length) * step, y: corner.y + (dy / length) * step }
}

/**
 * A rectangle rounded at the top two corners only — a box's header, which sits inside a box that is
 * rounded at all four. It is spelled as a path rather than clipped, so what the screen draws and
 * what the exported file holds are the same thing without a stylesheet in between.
 */
export function roundedTop(x: number, y: number, width: number, height: number, r = 5): string {
  const radius = Math.min(r, width / 2, height)
  return [
    `M ${x} ${y + radius}`,
    `A ${radius} ${radius} 0 0 1 ${x + radius} ${y}`,
    `H ${x + width - radius}`,
    `A ${radius} ${radius} 0 0 1 ${x + width} ${y + radius}`,
    `V ${y + height}`,
    `H ${x}`,
    'Z',
  ].join(' ')
}

export interface Rect {
  x: number
  y: number
  width: number
  height: number
}

export function overlaps(one: Rect, other: Rect): boolean {
  return (
    one.x < other.x + other.width &&
    one.x + one.width > other.x &&
    one.y < other.y + other.height &&
    one.y + one.height > other.y
  )
}

export function rectBetween(one: Point, other: Point): Rect {
  return {
    x: Math.min(one.x, other.x),
    y: Math.min(one.y, other.y),
    width: Math.abs(one.x - other.x),
    height: Math.abs(one.y - other.y),
  }
}

/** Everything the diagram covers, with room to breathe — what Fit and the export are measured by. */
export function bounds(boxes: readonly Box[], shapes: readonly DiagramShape[], pad = 40): Rect {
  const rects: Rect[] = [
    ...boxes.map((box) => ({ x: box.node.x, y: box.node.y, width: box.width, height: box.height })),
    ...shapes,
  ]
  const [first] = rects
  if (!first) return { x: 0, y: 0, width: 0, height: 0 }

  let left = first.x
  let top = first.y
  let right = first.x + first.width
  let bottom = first.y + first.height

  for (const rect of rects) {
    left = Math.min(left, rect.x)
    top = Math.min(top, rect.y)
    right = Math.max(right, rect.x + rect.width)
    bottom = Math.max(bottom, rect.y + rect.height)
  }

  return { x: left - pad, y: top - pad, width: right - left + pad * 2, height: bottom - top + pad * 2 }
}

/** A node the reader has just added, at a point they picked, with nothing read about it yet. */
export function newNode(table: string, at: Point): DiagramNode {
  return {
    table,
    x: Math.round(at.x),
    y: Math.round(at.y),
    width: DEFAULT_NODE_WIDTH,
    collapsed: false,
    colour: '',
  }
}

let shapeSeq = 0

export function newShape(kind: ShapeKind, rect: Rect): DiagramShape {
  shapeSeq += 1
  return {
    id: `shape-${shapeSeq}-${Math.round(rect.x)}`,
    kind,
    ...rect,
    text: kind === 'text' ? 'Text' : kind === 'note' ? 'Note' : '',
    colour: kind === 'note' ? 'yellow' : '',
  }
}

/** What a shape is called wherever one is named to a person. */
export const SHAPE_NOUN: Record<ShapeKind, string> = {
  note: 'Note',
  rectangle: 'Rectangle',
  ellipse: 'Ellipse',
  text: 'Text',
}

/** The smallest a drawn shape may be, so a stray click never leaves an invisible one behind. */
export const MIN_SHAPE = 24
