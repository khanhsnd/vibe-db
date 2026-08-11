import { useRef, type RefObject } from 'react'

import { ContextMenu, type MenuItem } from '@/shared/ui/ContextMenu'
import { useDiagramsStore, type DiagramView } from '@/stores/diagrams'

import { DiagramShapeView } from './DiagramShapeView'
import { RelationEdge } from './RelationEdge'
import { TableNode } from './TableNode'
import {
  GRID_STEP,
  nodeKey,
  pickedOf,
  relationKey,
  shapeKey,
  type Diagram,
  type LinkDraft,
  type Tool,
} from './model'
import { NOTE_SIZE } from './text'
import { useCanvasPointer, type Overlay } from './useCanvasPointer'
import type { DiagramData } from './useDiagram'

/**
 * The surface a diagram is drawn on: the grid behind it, the shapes, the lines, the boxes, and
 * whatever is being dragged over the lot.
 *
 * **Nothing here decides what a press means.** That is `useCanvasPointer`, which reads the `data-`
 * attributes these components carry and is the only thing that knows a drag from a click. This file
 * is the picture and the keyboard, and those are the two things a reader can see.
 */
interface DiagramCanvasProps {
  tabId: string
  diagram: Diagram
  view: DiagramView
  data: DiagramData
  /** Whether a relation may be drawn at all — see `EngineInfo.hasForeignKeys`. */
  linkable: boolean
  /** What this engine calls one table, for anything the canvas says about one. */
  word: string
  surface: RefObject<SVGSVGElement | null>
  onLink: (draft: LinkDraft) => void
  onOpenTable: (table: string) => void
  /** The menu for whatever was right-clicked, and for the canvas itself where that was nothing. */
  menuFor: (target: string | null) => MenuItem[]
}

/**
 * The letter each tool answers to, which is the shortcut every drawing surface there is uses for
 * that tool. They are the labels the toolbar draws in its tooltips, and nowhere else knows them.
 */
const TOOL_KEYS: Partial<Record<string, Tool>> = {
  v: 'select',
  h: 'pan',
  r: 'relation',
  n: 'note',
  b: 'rectangle',
  e: 'ellipse',
  t: 'text',
}

const NO_MARKS: ReadonlySet<string> = new Set()

export function DiagramCanvas({
  tabId,
  diagram,
  view,
  data,
  linkable,
  word,
  surface,
  onLink,
  onOpenTable,
  menuFor,
}: DiagramCanvasProps) {
  const change = useDiagramsStore((s) => s.change)
  const look = useDiagramsStore((s) => s.look)
  const undo = useDiagramsStore((s) => s.undo)
  const redo = useDiagramsStore((s) => s.redo)

  const frame = useRef<HTMLDivElement>(null)
  const pointer = useCanvasPointer({
    tabId,
    diagram,
    view,
    data,
    linkable,
    surface,
    frame,
    onLink,
    onOpenTable,
  })

  const pick = (keys: readonly string[]) => look(tabId, { selection: keys })

  const nudge = (dx: number, dy: number) => {
    const nodes = new Set(pickedOf(view.selection, 'node'))
    const shapes = new Set(pickedOf(view.selection, 'shape'))
    if (nodes.size === 0 && shapes.size === 0) return

    change(tabId, (one) => ({
      ...one,
      nodes: one.nodes.map((node) =>
        nodes.has(node.table) ? { ...node, x: node.x + dx, y: node.y + dy } : node,
      ),
      shapes: one.shapes.map((shape) =>
        shapes.has(shape.id) ? { ...shape, x: shape.x + dx, y: shape.y + dy } : shape,
      ),
    }))
  }

  /** Takes what is picked **off the diagram**. Nothing on the server is touched — see rule 19. */
  const remove = () => {
    const nodes = new Set(pickedOf(view.selection, 'node'))
    const shapes = new Set(pickedOf(view.selection, 'shape'))
    if (nodes.size === 0 && shapes.size === 0) return

    change(tabId, (one) => ({
      ...one,
      nodes: one.nodes.filter((node) => !nodes.has(node.table)),
      shapes: one.shapes.filter((shape) => !shapes.has(shape.id)),
    }))
    pick([])
  }

  const editing = diagram.shapes.find((shape) => shape.id === view.editing) ?? null

  // Which rows to mark: the ones the line under the pointer — or the one picked — actually lands on.
  const lit = pointer.hovered ?? pickedOf(view.selection, 'relation')[0] ?? null
  const marks = new Map<string, ReadonlySet<string>>()
  for (const relation of data.relations) {
    if (relation.id !== lit) continue
    marks.set(relation.from, new Set(relation.fromColumns))
    marks.set(relation.to, new Set(relation.toColumns))
  }

  return (
    <ContextMenu items={menuFor(pointer.menuTarget)}>
      <div
        ref={frame}
        tabIndex={0}
        className="relative h-full min-h-0 w-full overflow-hidden bg-canvas outline-none"
        onKeyDown={(event) => {
          if (view.editing !== null) return
          const key = event.key.toLowerCase()
          const tool = TOOL_KEYS[key]

          if (key === 'delete' || key === 'backspace') {
            event.preventDefault()
            remove()
          } else if (key === 'escape') {
            pick([])
            look(tabId, { tool: 'select' })
          } else if (key === 'z' && (event.ctrlKey || event.metaKey)) {
            event.preventDefault()
            if (event.shiftKey) redo(tabId)
            else undo(tabId)
          } else if (key === 'y' && (event.ctrlKey || event.metaKey)) {
            event.preventDefault()
            redo(tabId)
          } else if (key === 'a' && (event.ctrlKey || event.metaKey)) {
            event.preventDefault()
            pick([
              ...diagram.nodes.map((node) => nodeKey(node.table)),
              ...diagram.shapes.map((shape) => shapeKey(shape.id)),
            ])
          } else if (tool !== undefined && !event.ctrlKey && !event.metaKey) {
            look(tabId, { tool })
          } else if (key.startsWith('arrow')) {
            event.preventDefault()
            const step = event.shiftKey ? 1 : GRID_STEP
            const [dx, dy] =
              key === 'arrowleft'
                ? [-step, 0]
                : key === 'arrowright'
                  ? [step, 0]
                  : key === 'arrowup'
                    ? [0, -step]
                    : [0, step]
            nudge(dx, dy)
          }
        }}
      >
        <svg
          ref={surface}
          data-diagram=""
          className="h-full w-full"
          style={{ touchAction: 'none' }}
          onPointerDown={pointer.onPointerDown}
          onPointerMove={pointer.onPointerMove}
          onPointerUp={pointer.onPointerUp}
          onPointerCancel={pointer.onPointerUp}
          onDoubleClick={pointer.onDoubleClick}
          // No `onContextMenu` here on purpose. Radix composes its trigger's handler behind an
          // `event.defaultPrevented` check, so a child that refused the native menu first would
          // refuse *this* menu as well — and the right-click would open nothing at all.
        >
          <defs>
            <pattern
              id={`diagram-grid-${tabId}`}
              width={GRID_STEP * 2 * diagram.zoom}
              height={GRID_STEP * 2 * diagram.zoom}
              x={diagram.panX}
              y={diagram.panY}
              patternUnits="userSpaceOnUse"
            >
              <circle cx={0.5} cy={0.5} r={0.9} className="fill-line-strong" />
            </pattern>
          </defs>

          {/* Drawn in screen space rather than with the scene, so the dots stay a pixel across
              however far the picture is zoomed out. It is left out of an export for that reason. */}
          {diagram.grid ? (
            <rect data-grid="" width="100%" height="100%" fill={`url(#diagram-grid-${tabId})`} />
          ) : null}

          <g
            data-scene=""
            transform={`translate(${diagram.panX} ${diagram.panY}) scale(${diagram.zoom})`}
          >
            {/* Under the boxes always: a rectangle drawn round four tables is a background. */}
            {diagram.shapes.map((shape) => (
              <DiagramShapeView
                key={shape.id}
                shape={shape}
                selected={view.selection.includes(shapeKey(shape.id))}
                editing={view.editing === shape.id}
              />
            ))}

            {data.relations.map((relation) => {
              const from = data.byTable.get(relation.from)
              const to = data.byTable.get(relation.to)
              if (!from || !to) return null

              return (
                <RelationEdge
                  key={relation.id}
                  relation={relation}
                  from={from}
                  to={to}
                  selected={view.selection.includes(relationKey(relation.id))}
                  hovered={pointer.hovered === relation.id}
                  named={diagram.names}
                />
              )
            })}

            {data.boxes.map((box) => (
              <TableNode
                key={box.node.table}
                box={box}
                selected={view.selection.includes(nodeKey(box.node.table))}
                related={marks.has(box.node.table)}
                marked={marks.get(box.node.table) ?? NO_MARKS}
                linkable={linkable}
                word={word}
              />
            ))}

            <g data-overlay="">
              <Overlays overlay={pointer.overlay} />
            </g>
          </g>
        </svg>

        {/* A shape's words are typed into a real text box laid over it: SVG has no editable text,
            and a canvas that opened a dialog to change one word would not read as drawing. */}
        {editing ? (
          <textarea
            // By the shape, so moving from one note to another opens the second one's words rather
            // than leaving the first one's in an element React would otherwise reuse.
            key={editing.id}
            autoFocus
            defaultValue={editing.text}
            spellCheck={false}
            onBlur={(event) => {
              const text = event.target.value
              change(tabId, (one) => ({
                ...one,
                shapes: one.shapes.map((shape) =>
                  shape.id === editing.id ? { ...shape, text } : shape,
                ),
              }))
              look(tabId, { editing: null })
            }}
            onKeyDown={(event) => {
              if (event.key === 'Escape') event.currentTarget.blur()
              // Enter ends a one-line label and breaks a line in a note, which is what each is for.
              if (event.key === 'Enter' && (editing.kind === 'text' || event.ctrlKey)) {
                event.preventDefault()
                event.currentTarget.blur()
              }
            }}
            className="absolute resize-none rounded border border-accent bg-canvas p-1 text-fg outline-none"
            style={{
              left: editing.x * diagram.zoom + diagram.panX,
              top: editing.y * diagram.zoom + diagram.panY,
              width: editing.width * diagram.zoom,
              height: editing.height * diagram.zoom,
              fontFamily: 'var(--font-ui)',
              fontSize: NOTE_SIZE * diagram.zoom,
              lineHeight: 1.25,
            }}
          />
        ) : null}
      </div>
    </ContextMenu>
  )
}

/** The marquee, the line being drawn, or the shape being dragged out — whichever is happening. */
function Overlays({ overlay }: { overlay: Overlay | null }) {
  if (!overlay) return null

  if (overlay.kind === 'marquee') {
    return (
      <rect
        {...overlay.rect}
        className="fill-accent stroke-accent"
        fillOpacity={0.1}
        strokeDasharray="4 3"
        pointerEvents="none"
      />
    )
  }

  if (overlay.kind === 'draw') {
    return (
      <rect
        {...overlay.rect}
        rx={4}
        className="fill-accent stroke-accent"
        fillOpacity={0.08}
        strokeDasharray="4 3"
        pointerEvents="none"
      />
    )
  }

  return (
    <g pointerEvents="none">
      <path
        d={`M ${overlay.from.x} ${overlay.from.y} L ${overlay.to.x} ${overlay.to.y}`}
        fill="none"
        className={overlay.onto === '' ? 'stroke-faint' : 'stroke-kind-relation'}
        strokeWidth={1.5}
        strokeDasharray="5 4"
      />
      <circle
        cx={overlay.to.x}
        cy={overlay.to.y}
        r={4}
        className={overlay.onto === '' ? 'fill-faint' : 'fill-kind-relation'}
      />
    </g>
  )
}
