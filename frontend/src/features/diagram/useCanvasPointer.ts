import {
  useEffect,
  useRef,
  useState,
  type MouseEvent as ReactMouseEvent,
  type PointerEvent as ReactPointerEvent,
  type RefObject,
} from 'react'

import { useDiagramsStore, type DiagramView } from '@/stores/diagrams'

import {
  clampZoom,
  MIN_NODE_WIDTH,
  MIN_SHAPE,
  newShape,
  nodeKey,
  overlaps,
  pickedOf,
  rectBetween,
  relationKey,
  shapeKey,
  snapped,
  type Diagram,
  type LinkDraft,
  type Point,
  type Rect,
  type ShapeKind,
} from './model'
import type { DiagramData } from './useDiagram'

/**
 * Everything a pointer can do to a diagram, in one place.
 *
 * **One place reads a press.** The boxes, the lines and the notes draw themselves and carry `data-`
 * attributes saying what they are; this asks the pressed element what it was part of and decides
 * what the drag means. That is why a box knows nothing about selection and a line knows nothing
 * about dragging — and why a third kind of thing on the canvas is a case here rather than a second
 * interaction model.
 *
 * **A drag is one step.** The picture is written on every pointer move *without being recorded*, and
 * the state the drag started from is recorded once when the pointer comes up — so `Ctrl-Z` takes back
 * the drag rather than the last pixel of it. Panning and zooming are not steps at all: an undo that
 * put the scrolling back would undo the looking rather than the doing.
 */

/** What the pointer is in the middle of doing. Exactly one of these is live at a time. */
type Drag =
  | { kind: 'pan'; x: number; y: number; panX: number; panY: number }
  | { kind: 'move'; from: Point; nodes: Map<string, Point>; shapes: Map<string, Point> }
  | { kind: 'resizeNode'; table: string; from: number; width: number }
  | { kind: 'resizeShape'; id: string; from: Point; width: number; height: number }
  | { kind: 'marquee'; from: Point; add: boolean }
  | { kind: 'link'; table: string; column: string; from: Point }
  | { kind: 'draw'; shape: ShapeKind; from: Point }

/** What is drawn over the picture while a drag is running, and nothing while none is. */
export type Overlay =
  | { kind: 'marquee'; rect: Rect }
  | { kind: 'link'; from: Point; to: Point; onto: string }
  | { kind: 'draw'; shape: ShapeKind; rect: Rect }

const MIDDLE_BUTTON = 1
const RIGHT_BUTTON = 2

/**
 * What is actually under the pointer.
 *
 * It is not `event.target` while a drag is running: the canvas captures the pointer so a drag
 * survives leaving the window, and a captured pointer reports the **capture element** as its target.
 * A line dragged onto a box would otherwise never find the box.
 */
const under = (x: number, y: number): Element | null => document.elementFromPoint(x, y)

interface PointerInput {
  tabId: string
  diagram: Diagram
  view: DiagramView
  data: DiagramData
  /** Whether a line may be drawn at all — see `EngineInfo.hasForeignKeys`. */
  linkable: boolean
  surface: RefObject<SVGSVGElement | null>
  /** What takes the keyboard when the canvas is pressed. */
  frame: RefObject<HTMLDivElement | null>
  onLink: (draft: LinkDraft) => void
  onOpenTable: (table: string) => void
}

export interface CanvasPointer {
  overlay: Overlay | null
  /** The line under the pointer, which is what lights it and marks the rows at both ends. */
  hovered: string | null
  /** What was last right-clicked, and `null` for the canvas itself. */
  menuTarget: string | null
  onPointerDown: (event: ReactPointerEvent<SVGSVGElement>) => void
  onPointerMove: (event: ReactPointerEvent<SVGSVGElement>) => void
  onPointerUp: (event: ReactPointerEvent<SVGSVGElement>) => void
  onDoubleClick: (event: ReactMouseEvent<SVGSVGElement>) => void
}

export function useCanvasPointer({
  tabId,
  diagram,
  view,
  data,
  linkable,
  surface,
  frame,
  onLink,
  onOpenTable,
}: PointerInput): CanvasPointer {
  const change = useDiagramsStore((s) => s.change)
  const commit = useDiagramsStore((s) => s.commit)
  const look = useDiagramsStore((s) => s.look)

  const drag = useRef<Drag | null>(null)
  const before = useRef<Diagram | null>(null)
  const [overlay, setOverlay] = useState<Overlay | null>(null)
  const [hovered, setHovered] = useState<string | null>(null)
  const [menuTarget, setMenuTarget] = useState<string | null>(null)

  // The wheel is read through a listener of its own because React's is passive: a page that cannot
  // refuse Ctrl-wheel is a page whose zoom control is the browser's rather than the diagram's.
  const latest = useRef(diagram)
  latest.current = diagram

  useEffect(() => {
    const element = surface.current
    if (!element) return

    const onWheel = (event: WheelEvent) => {
      event.preventDefault()
      const held = latest.current

      if (event.ctrlKey || event.metaKey) {
        const rect = element.getBoundingClientRect()
        const zoom = clampZoom(held.zoom * (event.deltaY < 0 ? 1.12 : 1 / 1.12))
        const scale = zoom / held.zoom
        const x = event.clientX - rect.left
        const y = event.clientY - rect.top

        change(
          tabId,
          (one) => ({
            ...one,
            zoom,
            panX: x - (x - one.panX) * scale,
            panY: y - (y - one.panY) * scale,
          }),
          false,
        )
        return
      }

      // Shift turns a wheel sideways, which is what it does in every scrolling surface there is.
      const [dx, dy] = event.shiftKey ? [event.deltaY, 0] : [event.deltaX, event.deltaY]
      change(tabId, (one) => ({ ...one, panX: one.panX - dx, panY: one.panY - dy }), false)
    }

    element.addEventListener('wheel', onWheel, { passive: false })
    return () => element.removeEventListener('wheel', onWheel)
  }, [surface, change, tabId])

  const pointAt = (clientX: number, clientY: number): Point => {
    const rect = surface.current?.getBoundingClientRect()
    if (!rect) return { x: 0, y: 0 }
    return {
      x: (clientX - rect.left - diagram.panX) / diagram.zoom,
      y: (clientY - rect.top - diagram.panY) / diagram.zoom,
    }
  }

  const pick = (keys: readonly string[]) => look(tabId, { selection: keys })

  const toggle = (key: string): readonly string[] => {
    const next = view.selection.includes(key)
      ? view.selection.filter((one) => one !== key)
      : [...view.selection, key]
    pick(next)
    return next
  }

  const startMove = (keys: readonly string[], from: Point) => {
    const nodes = new Map<string, Point>()
    const shapes = new Map<string, Point>()

    for (const table of pickedOf(keys, 'node')) {
      const box = data.byTable.get(table)
      if (box) nodes.set(table, { x: box.node.x, y: box.node.y })
    }
    for (const id of pickedOf(keys, 'shape')) {
      const shape = diagram.shapes.find((one) => one.id === id)
      if (shape) shapes.set(id, { x: shape.x, y: shape.y })
    }
    if (nodes.size === 0 && shapes.size === 0) return

    drag.current = { kind: 'move', from, nodes, shapes }
  }

  const onPointerDown = (event: ReactPointerEvent<SVGSVGElement>) => {
    frame.current?.focus()

    const target = event.target as Element
    const port = target.closest('[data-port]')
    const shapeResize = target.closest('[data-shape-resize]')
    const nodeResize = target.closest('[data-resize]')
    const collapse = target.closest('[data-collapse]')
    const node = target.closest('[data-node]')?.getAttribute('data-node') ?? null
    const shape = target.closest('[data-shape]')?.getAttribute('data-shape') ?? null
    const relation = target.closest('[data-rel]')?.getAttribute('data-rel') ?? null

    const key = node
      ? nodeKey(node)
      : shape
        ? shapeKey(shape)
        : relation
          ? relationKey(relation)
          : null

    if (event.button === RIGHT_BUTTON) {
      // A right-click inside a pick of several leaves it standing — the rule the tree and the grid
      // both already follow, and the whole reason a pair survives being right-clicked.
      if (key && !view.selection.includes(key)) pick([key])
      if (!key) pick([])
      setMenuTarget(key)
      return
    }
    if (event.button !== 0 && event.button !== MIDDLE_BUTTON) return

    event.currentTarget.setPointerCapture(event.pointerId)
    before.current = diagram
    const at = pointAt(event.clientX, event.clientY)

    if (event.button === MIDDLE_BUTTON || view.tool === 'pan' || event.altKey) {
      drag.current = {
        kind: 'pan',
        x: event.clientX,
        y: event.clientY,
        panX: diagram.panX,
        panY: diagram.panY,
      }
      return
    }

    if (collapse && node) {
      change(tabId, (one) => ({
        ...one,
        nodes: one.nodes.map((held) =>
          held.table === node ? { ...held, collapsed: !held.collapsed } : held,
        ),
      }))
      return
    }

    if (port && node) {
      const column = port.getAttribute('data-field') ?? ''
      drag.current = { kind: 'link', table: node, column, from: at }
      setOverlay({ kind: 'link', from: at, to: at, onto: '' })
      return
    }

    if (shapeResize && shape) {
      const held = diagram.shapes.find((one) => one.id === shape)
      if (held) {
        pick([shapeKey(shape)])
        drag.current = {
          kind: 'resizeShape',
          id: shape,
          from: at,
          width: held.width,
          height: held.height,
        }
      }
      return
    }

    if (nodeResize && node) {
      const box = data.byTable.get(node)
      if (box) {
        pick([nodeKey(node)])
        drag.current = { kind: 'resizeNode', table: node, from: at.x, width: box.node.width }
      }
      return
    }

    // The relation tool turns the whole of a field row into a place a line starts, so a reader who
    // has chosen to draw one does not also have to find a four-pixel handle.
    if (view.tool === 'relation' && node && linkable) {
      const column = target.closest('[data-field]')?.getAttribute('data-field') ?? ''
      drag.current = { kind: 'link', table: node, column, from: at }
      setOverlay({ kind: 'link', from: at, to: at, onto: '' })
      return
    }

    if (key !== null) {
      // A plain press on something already picked leaves the pick alone, so a group of boxes is
      // dragged as one rather than collapsing to whichever of them was pressed.
      const adding = event.shiftKey || event.ctrlKey || event.metaKey
      const picked = adding
        ? toggle(key)
        : view.selection.includes(key)
          ? view.selection
          : (pick([key]), [key])

      if (relation === null) startMove(picked, at)
      return
    }

    if (view.tool !== 'select' && view.tool !== 'relation') {
      drag.current = { kind: 'draw', shape: view.tool, from: at }
      setOverlay({ kind: 'draw', shape: view.tool, rect: { ...at, width: 0, height: 0 } })
      return
    }

    drag.current = { kind: 'marquee', from: at, add: event.shiftKey }
    setOverlay({ kind: 'marquee', rect: { ...at, width: 0, height: 0 } })
  }

  const onPointerMove = (event: ReactPointerEvent<SVGSVGElement>) => {
    const held = drag.current
    if (!held) {
      const over = (event.target as Element).closest('[data-rel]')?.getAttribute('data-rel') ?? null
      if (over !== hovered) setHovered(over)
      return
    }

    if (held.kind === 'pan') {
      change(
        tabId,
        (one) => ({
          ...one,
          panX: held.panX + (event.clientX - held.x),
          panY: held.panY + (event.clientY - held.y),
        }),
        false,
      )
      return
    }

    const at = pointAt(event.clientX, event.clientY)

    switch (held.kind) {
      case 'move': {
        const dx = at.x - held.from.x
        const dy = at.y - held.from.y
        change(
          tabId,
          (one) => ({
            ...one,
            nodes: one.nodes.map((node) => {
              const origin = held.nodes.get(node.table)
              if (!origin) return node
              return {
                ...node,
                x: snapped(origin.x + dx, one.snap),
                y: snapped(origin.y + dy, one.snap),
              }
            }),
            shapes: one.shapes.map((shape) => {
              const origin = held.shapes.get(shape.id)
              if (!origin) return shape
              return {
                ...shape,
                x: snapped(origin.x + dx, one.snap),
                y: snapped(origin.y + dy, one.snap),
              }
            }),
          }),
          false,
        )
        return
      }

      case 'resizeNode': {
        const width = Math.max(MIN_NODE_WIDTH, held.width + (at.x - held.from))
        change(
          tabId,
          (one) => ({
            ...one,
            nodes: one.nodes.map((node) =>
              node.table === held.table ? { ...node, width: snapped(width, one.snap) } : node,
            ),
          }),
          false,
        )
        return
      }

      case 'resizeShape': {
        change(
          tabId,
          (one) => ({
            ...one,
            shapes: one.shapes.map((shape) =>
              shape.id === held.id
                ? {
                    ...shape,
                    width: Math.max(MIN_SHAPE, snapped(held.width + (at.x - held.from.x), one.snap)),
                    height: Math.max(
                      MIN_SHAPE,
                      snapped(held.height + (at.y - held.from.y), one.snap),
                    ),
                  }
                : shape,
            ),
          }),
          false,
        )
        return
      }

      case 'link': {
        const over = under(event.clientX, event.clientY)?.closest('[data-node]')
        setOverlay({
          kind: 'link',
          from: held.from,
          to: at,
          onto: over?.getAttribute('data-node') ?? '',
        })
        return
      }

      case 'marquee':
        setOverlay({ kind: 'marquee', rect: rectBetween(held.from, at) })
        return

      case 'draw':
        setOverlay({ kind: 'draw', shape: held.shape, rect: rectBetween(held.from, at) })
        return
    }
  }

  const onPointerUp = (event: ReactPointerEvent<SVGSVGElement>) => {
    const held = drag.current
    drag.current = null
    setOverlay(null)
    if (!held) return

    const at = pointAt(event.clientX, event.clientY)

    if (held.kind === 'marquee') {
      const rect = rectBetween(held.from, at)
      const caught = [
        ...data.boxes
          .filter((box) =>
            overlaps(rect, { x: box.node.x, y: box.node.y, width: box.width, height: box.height }),
          )
          .map((box) => nodeKey(box.node.table)),
        ...diagram.shapes
          .filter((shape) => overlaps(rect, shape))
          .map((shape) => shapeKey(shape.id)),
      ]
      pick(held.add ? [...new Set([...view.selection, ...caught])] : caught)
      return
    }

    if (held.kind === 'draw') {
      const rect = rectBetween(held.from, at)
      // A press that never moved is still a shape: it takes the size the tool ships with, so a
      // click with the Note tool leaves a note rather than nothing at all.
      const size = defaultSize(held.shape)
      const shape = newShape(held.shape, {
        x: snapped(rect.x, diagram.snap),
        y: snapped(rect.y, diagram.snap),
        width: Math.max(rect.width < MIN_SHAPE ? size.width : rect.width, MIN_SHAPE),
        height: Math.max(rect.height < MIN_SHAPE ? size.height : rect.height, MIN_SHAPE),
      })

      change(tabId, (one) => ({ ...one, shapes: [...one.shapes, shape] }))
      // A note and a label are drawn *to be written in*, so the caret is already there; a rectangle
      // round four tables usually says what it means by being where it is.
      look(tabId, {
        selection: [shapeKey(shape.id)],
        tool: 'select',
        editing: shape.kind === 'note' || shape.kind === 'text' ? shape.id : null,
      })
      return
    }

    if (held.kind === 'link') {
      const target = under(event.clientX, event.clientY)
      const onto = target?.closest('[data-node]')?.getAttribute('data-node') ?? ''
      const column = target?.closest('[data-field]')?.getAttribute('data-field') ?? ''

      if (onto !== '' && !(onto === held.table && column === held.column)) {
        onLink({ from: held.table, fromColumn: held.column, to: onto, toColumn: column })
      }
      return
    }

    // Everything else wrote the picture as it went, so the step to come back to is where it started.
    if (before.current) commit(tabId, before.current)
    before.current = null
  }

  const onDoubleClick = (event: ReactMouseEvent<SVGSVGElement>) => {
    const target = event.target as Element
    const shape = target.closest('[data-shape]')?.getAttribute('data-shape')
    if (shape) {
      look(tabId, { editing: shape, selection: [shapeKey(shape)] })
      return
    }

    const node = target.closest('[data-node]')?.getAttribute('data-node')
    if (node) onOpenTable(node)
  }

  return { overlay, hovered, menuTarget, onPointerDown, onPointerMove, onPointerUp, onDoubleClick }
}

/** What a shape is when it was clicked for rather than dragged out. */
function defaultSize(kind: ShapeKind): { width: number; height: number } {
  if (kind === 'text') return { width: 140, height: 24 }
  if (kind === 'note') return { width: 180, height: 110 }
  return { width: 220, height: 140 }
}
