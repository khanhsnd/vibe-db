import { GRID_STEP, type Box, type DiagramNode, type Point, type Rect, type Relation } from './model'

/**
 * Where the boxes go when the reader asks for them to be arranged.
 *
 * A schema is a graph, and the one arrangement that reads as *this table points at that one* is by
 * **depth**: a table nothing points at from below sits in the first column, a table that points at
 * it in the second, and so on — so every line runs the same way across the page. A cycle has no
 * depth, so one is broken by refusing to count an edge twice; the pair simply share a column.
 *
 * Inside a column the order is the **average height of what each box is joined to** in the column
 * before it, which is the one cheap trick that keeps lines from crossing: a box sits opposite the
 * things it points at.
 */

/** The space between two columns, and between two boxes in one. */
const COLUMN_GAP = 90
const ROW_GAP = 32

export function arranged(
  boxes: readonly Box[],
  relations: readonly Relation[],
  origin = { x: 40, y: 40 },
): DiagramNode[] {
  if (boxes.length === 0) return []

  const parents = new Map<string, string[]>()
  for (const box of boxes) parents.set(box.node.table, [])
  for (const relation of relations) {
    if (relation.from === relation.to) continue
    parents.get(relation.from)?.push(relation.to)
  }

  const depths = new Map<string, number>()
  const walking = new Set<string>()

  const depthOf = (table: string): number => {
    const held = depths.get(table)
    if (held !== undefined) return held
    // A cycle counts as the bottom: neither of the pair can be below the other, so they share a
    // column and the lines between them run vertically rather than nowhere.
    if (walking.has(table)) return 0

    walking.add(table)
    let depth = 0
    for (const parent of parents.get(table) ?? []) {
      if (parent === table) continue
      depth = Math.max(depth, depthOf(parent) + 1)
    }
    walking.delete(table)

    depths.set(table, depth)
    return depth
  }

  const columns: Box[][] = []
  for (const box of boxes) {
    const depth = depthOf(box.node.table)
    const column = (columns[depth] ??= [])
    column.push(box)
  }

  const placed = new Map<string, { x: number; y: number }>()
  let x = origin.x

  for (const column of columns) {
    if (!column) continue

    const ordered = [...column].sort(
      (one, other) => weightOf(one, relations, placed) - weightOf(other, relations, placed),
    )

    let y = origin.y
    for (const box of ordered) {
      placed.set(box.node.table, { x: round(x), y: round(y) })
      y += box.height + ROW_GAP
    }

    const widest = column.reduce((most, box) => Math.max(most, box.width), 0)
    x += widest + COLUMN_GAP
  }

  return boxes.map((box) => {
    const at = placed.get(box.node.table)
    return at ? { ...box.node, ...at } : box.node
  })
}

// How high up a box wants to sit: the average height of everything already placed that it is joined
// to. A box joined to nothing placed keeps its turn, which is what the large number stands for.
function weightOf(
  box: Box,
  relations: readonly Relation[],
  placed: Map<string, { x: number; y: number }>,
): number {
  const heights: number[] = []

  for (const relation of relations) {
    const other =
      relation.from === box.node.table
        ? relation.to
        : relation.to === box.node.table
          ? relation.from
          : ''
    const at = other === '' ? undefined : placed.get(other)
    if (at) heights.push(at.y)
  }

  if (heights.length === 0) return Number.MAX_SAFE_INTEGER
  return heights.reduce((sum, height) => sum + height, 0) / heights.length
}

const round = (value: number) => Math.round(value / GRID_STEP) * GRID_STEP

/**
 * Where a table dropped with no room asked for goes: down the first free slot of a coarse grid, so
 * adding six tables from the palette in a row does not stack all six in one place.
 */
export function freeSpot(taken: readonly Rect[], width: number, height: number, from: Point): Point {
  const step = GRID_STEP * 2
  let at = { ...from }

  for (let tries = 0; tries < 400; tries += 1) {
    const clash = taken.some(
      (rect) =>
        at.x < rect.x + rect.width + step &&
        at.x + width + step > rect.x &&
        at.y < rect.y + rect.height + step &&
        at.y + height + step > rect.y,
    )
    if (!clash) return at
    at = { x: at.x + step, y: at.y + step }
  }
  return at
}

/** A box as a plain rectangle, which is all anything about *where things are* needs of one. */
export const rectOf = (box: Box): Rect => ({
  x: box.node.x,
  y: box.node.y,
  width: box.width,
  height: box.height,
})
