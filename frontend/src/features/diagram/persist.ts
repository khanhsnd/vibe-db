import type { Namespace } from '@/bridge/connections'

import {
  DEFAULT_NODE_WIDTH,
  EMPTY_DIAGRAM,
  clampZoom,
  type Diagram,
  type DiagramNode,
  type DiagramShape,
  type DisplayMode,
  type ShapeKind,
} from './model'

/**
 * The diagram drawn for one namespace, kept where the reader left it.
 *
 * It is stored the way a table's column order is — keyed by what it is *about*, beside the code that
 * reads it, and never on the backend: where a box sits is a reading preference and no server has any
 * business hearing about it. What is stored is only what the reader put there; the fields, the keys
 * and the lines are read from the catalogue every time, so a diagram cannot go stale.
 */

const STORE_KEY = 'vibe-db.diagrams'
const SAVE_DELAY_MS = 400

/** What names one diagram: which connection, and which namespace of it. */
export const diagramKey = (profileId: string, namespace: Namespace) =>
  `${profileId}:${namespace.database}.${namespace.schema}`

function stored(): Record<string, unknown> {
  const raw = localStorage.getItem(STORE_KEY)
  if (raw === null) return {}

  try {
    const parsed = JSON.parse(raw) as unknown
    return typeof parsed === 'object' && parsed !== null ? (parsed as Record<string, unknown>) : {}
  } catch {
    return {}
  }
}

export function loadDiagram(key: string): Diagram {
  return readDiagram(stored()[key])
}

/**
 * Reads a field at a time rather than trusting the entry whole: what is in there was written by a
 * version of this app, and a later one writes something else. Anything that does not read as a
 * diagram falls back to an empty one, which is a canvas the reader can draw on rather than a tab
 * that opens on nothing.
 */
function readDiagram(held: unknown): Diagram {
  if (typeof held !== 'object' || held === null) return EMPTY_DIAGRAM
  const one = held as Record<string, unknown>

  return {
    nodes: Array.isArray(one.nodes) ? one.nodes.flatMap(readNode) : [],
    shapes: Array.isArray(one.shapes) ? one.shapes.flatMap(readShape) : [],
    display: readDisplay(one.display),
    snap: one.snap !== false,
    grid: one.grid !== false,
    names: one.names === true,
    zoom: typeof one.zoom === 'number' ? clampZoom(one.zoom) : EMPTY_DIAGRAM.zoom,
    panX: typeof one.panX === 'number' ? one.panX : EMPTY_DIAGRAM.panX,
    panY: typeof one.panY === 'number' ? one.panY : EMPTY_DIAGRAM.panY,
  }
}

function readNode(held: unknown): DiagramNode[] {
  if (typeof held !== 'object' || held === null) return []
  const one = held as Record<string, unknown>
  if (typeof one.table !== 'string' || one.table === '') return []

  return [
    {
      table: one.table,
      x: number(one.x, 0),
      y: number(one.y, 0),
      width: number(one.width, DEFAULT_NODE_WIDTH),
      collapsed: one.collapsed === true,
      colour: typeof one.colour === 'string' ? one.colour : '',
    },
  ]
}

const SHAPE_KINDS: readonly ShapeKind[] = ['note', 'rectangle', 'ellipse', 'text']

function readShape(held: unknown): DiagramShape[] {
  if (typeof held !== 'object' || held === null) return []
  const one = held as Record<string, unknown>
  if (typeof one.id !== 'string' || one.id === '') return []

  const kind = SHAPE_KINDS.find((value) => value === one.kind)
  if (!kind) return []

  return [
    {
      id: one.id,
      kind,
      x: number(one.x, 0),
      y: number(one.y, 0),
      width: number(one.width, 160),
      height: number(one.height, 90),
      text: typeof one.text === 'string' ? one.text : '',
      colour: typeof one.colour === 'string' ? one.colour : '',
    },
  ]
}

const DISPLAYS: readonly DisplayMode[] = ['full', 'keys', 'title']

function readDisplay(held: unknown): DisplayMode {
  return DISPLAYS.find((value) => value === held) ?? 'full'
}

const number = (held: unknown, fallback: number) =>
  typeof held === 'number' && Number.isFinite(held) ? held : fallback

let pending: ReturnType<typeof setTimeout> | undefined
const latest = new Map<string, Diagram>()

/**
 * Writes a diagram down on its own clock rather than on the drag's: a box being moved changes it
 * sixty times a second, and every one of those would otherwise be a `JSON.stringify` of the whole
 * picture. What lands is always the latest state of every diagram touched since the last write.
 */
export function saveDiagram(key: string, diagram: Diagram): void {
  latest.set(key, diagram)
  if (pending === undefined) pending = setTimeout(flush, SAVE_DELAY_MS)
}

function flush(): void {
  pending = undefined
  if (latest.size === 0) return

  const held = stored()
  for (const [key, diagram] of latest) held[key] = diagram
  latest.clear()
  localStorage.setItem(STORE_KEY, JSON.stringify(held))
}

window.addEventListener('beforeunload', () => {
  if (pending === undefined) return
  clearTimeout(pending)
  flush()
})
