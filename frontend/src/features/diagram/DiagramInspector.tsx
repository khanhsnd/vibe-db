import {
  AlignHorizontalJustifyCenter,
  AlignVerticalJustifyCenter,
  Columns3,
  MoveHorizontal,
  MoveVertical,
  Spline,
  Table2,
  Trash2,
} from 'lucide-react'

import { CONNECTION_COLOURS } from '@/bridge/connections'
import { cn } from '@/lib/cn'
import { Button } from '@/shared/ui/Button'
import { TextArea } from '@/shared/ui/Field'
import { IconButton } from '@/shared/ui/IconButton'
import { useDiagramsStore, type DiagramView } from '@/stores/diagrams'

import {
  nodeKey,
  pickedOf,
  relationKey,
  SHAPE_NOUN,
  type Diagram,
  type DiagramShape,
  type Point,
  type Relation,
} from './model'

/** One thing being lined up, whichever kind it is: everything aligning needs and nothing more. */
interface Placed extends Point {
  id: string
  width: number
  height: number
}
import { fieldType, fitWidth } from './text'
import type { DiagramData } from './useDiagram'

/**
 * What is picked, spelled out.
 *
 * The canvas says where things are and this says what they *are*: which constraint a line stands
 * for, what a box's colour is, how many fields it has. It is also the one place a **picked pair** is
 * worth anything — lining four boxes up is an action on the selection and on nothing else — which is
 * why aligning lives here rather than in a toolbar that would have to grey it out.
 */
interface DiagramInspectorProps {
  tabId: string
  diagram: Diagram
  view: DiagramView
  data: DiagramData
  /** What this engine calls a table and a row. */
  words: { table: string; rows: string }
  onOpenTable: (table: string) => void
  onDesignTable: (table: string) => void
  onDropRelation: (relation: Relation) => void
}

export function DiagramInspector({
  tabId,
  diagram,
  view,
  data,
  words,
  onOpenTable,
  onDesignTable,
  onDropRelation,
}: DiagramInspectorProps) {
  const change = useDiagramsStore((s) => s.change)
  const look = useDiagramsStore((s) => s.look)

  const nodes = pickedOf(view.selection, 'node')
  const shapes = pickedOf(view.selection, 'shape')
  const relations = pickedOf(view.selection, 'relation')
  const picked = nodes.length + shapes.length + relations.length

  const align = (edit: (moving: readonly Placed[]) => Map<string, Point>) => {
    const boxes = nodes.flatMap((table): Placed[] => {
      const box = data.byTable.get(table)
      if (!box) return []
      return [{ id: nodeKey(table), x: box.node.x, y: box.node.y, width: box.width, height: box.height }]
    })
    const drawn = shapes.flatMap((id): Placed[] => {
      const shape = diagram.shapes.find((one) => one.id === id)
      if (!shape) return []
      return [{ id: `shape:${id}`, x: shape.x, y: shape.y, width: shape.width, height: shape.height }]
    })

    const all = [...boxes, ...drawn]
    if (all.length < 2) return

    const moved = edit(all)
    change(tabId, (one) => ({
      ...one,
      nodes: one.nodes.map((node) => {
        const at = moved.get(nodeKey(node.table))
        return at ? { ...node, ...at } : node
      }),
      shapes: one.shapes.map((shape) => {
        const at = moved.get(`shape:${shape.id}`)
        return at ? { ...shape, ...at } : shape
      }),
    }))
  }

  return (
    <div className="scroll-thin flex h-full min-h-0 flex-col gap-3 overflow-y-auto border-l border-line bg-panel p-3 text-xs">
      {picked === 0 ? (
        <Summary diagram={diagram} data={data} words={words} />
      ) : picked > 1 ? (
        <Several
          count={picked}
          onAlign={(edge) =>
            align((all) => {
              const moved = new Map<string, Point>()
              const left = Math.min(...all.map((one) => one.x))
              const top = Math.min(...all.map((one) => one.y))

              for (const one of all) {
                moved.set(one.id, {
                  x: edge === 'left' ? left : one.x,
                  y: edge === 'top' ? top : one.y,
                })
              }
              return moved
            })
          }
          onSpread={(axis) =>
            align((all) => {
              const moved = new Map<string, Point>()
              const order = [...all].sort((one, other) =>
                axis === 'x' ? one.x - other.x : one.y - other.y,
              )

              // Evenly, from where the first and last already are: the reader placed the ends, and
              // what they are asking for is the same gap between everything in between.
              const first = order[0]
              const last = order.at(-1)
              if (!first || !last) return moved

              const span = axis === 'x' ? last.x - first.x : last.y - first.y
              const step = span / Math.max(1, order.length - 1)

              order.forEach((one, at) => {
                moved.set(one.id, {
                  x: axis === 'x' ? Math.round(first.x + step * at) : one.x,
                  y: axis === 'y' ? Math.round(first.y + step * at) : one.y,
                })
              })
              return moved
            })
          }
        />
      ) : relations.length === 1 ? (
        <RelationFacts
          relation={data.relations.find((one) => one.id === relations[0]) ?? null}
          onDrop={onDropRelation}
        />
      ) : shapes.length === 1 ? (
        <ShapeFacts
          shape={diagram.shapes.find((one) => one.id === shapes[0]) ?? null}
          onText={(id, text) =>
            change(tabId, (one) => ({
              ...one,
              shapes: one.shapes.map((shape) => (shape.id === id ? { ...shape, text } : shape)),
            }))
          }
          onColour={(id, colour) =>
            change(tabId, (one) => ({
              ...one,
              shapes: one.shapes.map((shape) => (shape.id === id ? { ...shape, colour } : shape)),
            }))
          }
        />
      ) : (
        <NodeFacts
          table={nodes[0] ?? ''}
          data={data}
          words={words}
          onColour={(table, colour) =>
            change(tabId, (one) => ({
              ...one,
              nodes: one.nodes.map((node) => (node.table === table ? { ...node, colour } : node)),
            }))
          }
          onFit={(table) => {
            const box = data.byTable.get(table)
            if (!box) return
            const width = fitWidth(table, data.structureOf(table)?.columns ?? [])
            change(tabId, (one) => ({
              ...one,
              nodes: one.nodes.map((node) => (node.table === table ? { ...node, width } : node)),
            }))
          }}
          onPickRelation={(id) => look(tabId, { selection: [relationKey(id)] })}
          onOpen={onOpenTable}
          onDesign={onDesignTable}
        />
      )}
    </div>
  )
}

function Heading({ children }: { children: string }) {
  return <div className="text-2xs font-semibold tracking-wide text-faint uppercase">{children}</div>
}

function Fact({ label, value }: { label: string; value: string }) {
  return (
    <div className="flex items-baseline justify-between gap-2">
      <span className="text-muted">{label}</span>
      <span className="selectable min-w-0 truncate text-right font-mono text-2xs text-fg">
        {value}
      </span>
    </div>
  )
}

function Summary({
  diagram,
  data,
  words,
}: {
  diagram: Diagram
  data: DiagramData
  words: { table: string; rows: string }
}) {
  return (
    <>
      <Heading>Diagram</Heading>
      <Fact label={words.table === 'Table' ? 'Tables' : `${words.table}s`} value={String(diagram.nodes.length)} />
      <Fact label="Relations" value={String(data.relations.length)} />
      <Fact label="Notes and shapes" value={String(diagram.shapes.length)} />
      <p className="mt-1 leading-relaxed text-faint">
        Pick a box, a line or a note to see what it is. Drag a table in from the list on the left,
        and drag from the round handle on a field onto another table to declare a foreign key.
      </p>
    </>
  )
}

function Several({
  count,
  onAlign,
  onSpread,
}: {
  count: number
  onAlign: (edge: 'left' | 'top') => void
  onSpread: (axis: 'x' | 'y') => void
}) {
  return (
    <>
      <Heading>{`${count} selected`}</Heading>
      <div className="flex flex-wrap gap-1">
        <IconButton
          label="Line up the left edges"
          icon={<AlignHorizontalJustifyCenter size={14} />}
          onClick={() => onAlign('left')}
        />
        <IconButton
          label="Line up the top edges"
          icon={<AlignVerticalJustifyCenter size={14} />}
          onClick={() => onAlign('top')}
        />
        <IconButton
          label="Space them evenly across"
          icon={<MoveHorizontal size={14} />}
          onClick={() => onSpread('x')}
        />
        <IconButton
          label="Space them evenly down"
          icon={<MoveVertical size={14} />}
          onClick={() => onSpread('y')}
        />
      </div>
      <p className="leading-relaxed text-faint">
        Drag any one of them to move the whole pick. Delete takes them off the diagram; nothing on
        the server is touched.
      </p>
    </>
  )
}

function Swatches({
  colour,
  onPick,
}: {
  colour: string
  onPick: (colour: string) => void
}) {
  return (
    <div className="flex flex-wrap gap-1">
      {CONNECTION_COLOURS.map((one) => (
        <button
          key={one.value}
          type="button"
          title={one.label}
          aria-label={one.label}
          onClick={() => onPick(one.value)}
          className={cn(
            'h-4 w-4 rounded border',
            colour === one.value ? 'border-accent' : 'border-line',
          )}
          style={one.colour === '' ? undefined : { backgroundColor: one.colour }}
        >
          {one.colour === '' ? <span className="text-2xs text-faint">·</span> : null}
        </button>
      ))}
    </div>
  )
}

interface NodeFactsProps {
  table: string
  data: DiagramData
  words: { table: string; rows: string }
  onColour: (table: string, colour: string) => void
  onFit: (table: string) => void
  onPickRelation: (id: string) => void
  onOpen: (table: string) => void
  onDesign: (table: string) => void
}

function NodeFacts({
  table,
  data,
  words,
  onColour,
  onFit,
  onPickRelation,
  onOpen,
  onDesign,
}: NodeFactsProps) {
  const box = data.byTable.get(table)
  const structure = data.structureOf(table)
  const joined = data.relations.filter((one) => one.from === table || one.to === table)

  if (!box) return <Heading>Nothing</Heading>

  return (
    <>
      <div className="flex items-center gap-1.5">
        <Table2 size={13} className="shrink-0 text-kind-table" />
        <span className="selectable min-w-0 truncate text-sm font-medium text-fg">{table}</span>
      </div>

      <Fact label="Fields" value={String(structure?.columns.length ?? 0)} />
      <Fact label="Indexes" value={String(structure?.indexes.length ?? 0)} />
      <Fact label="Foreign keys" value={String(structure?.foreignKeys.length ?? 0)} />
      <Fact label="Width" value={`${Math.round(box.width)} px`} />

      <Heading>Colour</Heading>
      <Swatches colour={box.node.colour} onPick={(colour) => onColour(table, colour)} />

      <Heading>Keys</Heading>
      <div className="flex flex-col gap-0.5">
        {(structure?.columns ?? [])
          .filter((column) => column.primaryKey || box.foreign.has(column.name))
          .map((column) => (
            <div key={column.name} className="flex items-baseline justify-between gap-2">
              <span className={column.primaryKey ? 'text-fg' : 'text-muted'}>{column.name}</span>
              <span className="min-w-0 truncate font-mono text-2xs text-faint">
                {fieldType(column)}
              </span>
            </div>
          ))}
        {(structure?.columns ?? []).every(
          (column) => !column.primaryKey && !box.foreign.has(column.name),
        ) ? (
          <span className="text-faint">No key of any kind.</span>
        ) : null}
      </div>

      <Heading>{`Relations (${joined.length})`}</Heading>
      <div className="flex flex-col gap-0.5">
        {joined.map((relation) => (
          <button
            key={relation.id}
            type="button"
            onClick={() => onPickRelation(relation.id)}
            className="flex items-center gap-1.5 rounded px-1 py-0.5 text-left hover:bg-hover"
          >
            <Spline size={11} className="shrink-0 text-kind-relation" />
            <span className="min-w-0 flex-1 truncate text-muted">
              {relation.from === table ? `→ ${relation.to}` : `← ${relation.from}`}
            </span>
            <span className="min-w-0 max-w-[45%] truncate font-mono text-2xs text-faint">
              {relation.name}
            </span>
          </button>
        ))}
        {joined.length === 0 ? <span className="text-faint">Joined to nothing drawn.</span> : null}
      </div>

      <div className="mt-1 flex flex-wrap gap-1.5">
        <Button onClick={() => onOpen(table)} icon={<Table2 size={13} />}>
          {`Open ${words.table}`}
        </Button>
        <Button onClick={() => onDesign(table)} icon={<Columns3 size={13} />}>
          Design
        </Button>
        <Button variant="subtle" onClick={() => onFit(table)}>
          Fit width
        </Button>
      </div>
    </>
  )
}

function ShapeFacts({
  shape,
  onText,
  onColour,
}: {
  shape: DiagramShape | null
  onText: (id: string, text: string) => void
  onColour: (id: string, colour: string) => void
}) {
  if (!shape) return <Heading>Nothing</Heading>

  return (
    <>
      <Heading>{SHAPE_NOUN[shape.kind]}</Heading>
      <TextArea
        rows={5}
        value={shape.text}
        placeholder="What this says"
        onChange={(event) => onText(shape.id, event.target.value)}
      />
      <Heading>Colour</Heading>
      <Swatches colour={shape.colour} onPick={(colour) => onColour(shape.id, colour)} />
      <Fact label="Size" value={`${Math.round(shape.width)} × ${Math.round(shape.height)}`} />
      <p className="leading-relaxed text-faint">
        Double-click it on the canvas to type into it, and drag the corner to resize it. It belongs
        to the diagram alone — no server has ever heard of it.
      </p>
    </>
  )
}

function RelationFacts({
  relation,
  onDrop,
}: {
  relation: Relation | null
  onDrop: (relation: Relation) => void
}) {
  if (!relation) return <Heading>Nothing</Heading>

  return (
    <>
      <div className="flex items-center gap-1.5">
        <Spline size={13} className="shrink-0 text-kind-relation" />
        <span className="selectable min-w-0 truncate text-sm font-medium text-fg">
          {relation.name}
        </span>
      </div>

      <Fact label="From" value={`${relation.from} (${relation.fromColumns.join(', ')})`} />
      <Fact label="References" value={`${relation.to} (${relation.toColumns.join(', ')})`} />
      <Fact label="On update" value={relation.onUpdate || 'NO ACTION'} />
      <Fact label="On delete" value={relation.onDelete || 'NO ACTION'} />
      <Fact label="Optional" value={relation.optional ? 'Yes — every column may be null' : 'No'} />

      <p className="leading-relaxed text-faint">
        The crow’s foot is at the table that holds the constraint, which is the many side. A circle
        at the other end means the key may be null, so a row there need not point at anything.
      </p>

      <div className="mt-1">
        <Button variant="danger" icon={<Trash2 size={13} />} onClick={() => onDrop(relation)}>
          Delete Foreign Key
        </Button>
      </div>
    </>
  )
}
