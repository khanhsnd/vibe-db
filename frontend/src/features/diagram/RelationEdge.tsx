import { cn } from '@/lib/cn'

import {
  anchorOf,
  elbow,
  facing,
  polyline,
  ROW_HEIGHT,
  STUB,
  type Anchor,
  type Box,
  type Point,
  type Relation,
} from './model'
import { canvasFont, ellipsed, FIELD_SIZE } from './text'

/**
 * One foreign key, drawn between the two boxes it joins.
 *
 * The notation is the one every ER diagram uses and is *read from the constraint itself*: the crow's
 * foot is at the table that **holds** the key, because that is the many side, and the bar is at the
 * table it points at. A key every column of which may be null is optional, so it wears the circle —
 * which means the reader can tell a required relation from an optional one without opening anything.
 */
interface RelationEdgeProps {
  relation: Relation
  from: Box
  to: Box
  selected: boolean
  hovered: boolean
  /** Whether the constraint's own name is drawn along the line. */
  named: boolean
}

export function RelationEdge({ relation, from, to, selected, hovered, named }: RelationEdgeProps) {
  const { start, end, points } = geometry(relation, from, to)
  const path = polyline(points)
  const lit = selected || hovered

  return (
    <g data-rel={relation.id} style={{ cursor: 'pointer' }}>
      {/* A one-pixel line is a one-pixel target, so the thing the pointer actually hits is this. */}
      <path d={path} fill="none" stroke="transparent" strokeWidth={12} />

      <path
        d={path}
        fill="none"
        className={cn(selected ? 'stroke-accent' : hovered ? 'stroke-fg' : 'stroke-kind-relation')}
        strokeWidth={lit ? 2 : 1.25}
        strokeLinejoin="round"
        opacity={lit ? 1 : 0.85}
      />

      <CrowsFoot at={start} lit={lit} />
      <OneEnd at={end} optional={relation.optional} lit={lit} />

      {named ? <Label relation={relation} points={points} /> : null}
    </g>
  )
}

/** Where the line runs, and which end is which. A key pointing at its own table loops beside it. */
function geometry(relation: Relation, from: Box, to: Box) {
  if (relation.from === relation.to) {
    const start = anchorOf(from, relation.fromColumns[0], 1)
    const end = anchorOf(to, relation.toColumns[0], 1)
    const apart = Math.abs(start.y - end.y) < 1 ? ROW_HEIGHT : 0

    return {
      start,
      end,
      points: [
        { x: start.x, y: start.y },
        { x: start.x + STUB * 1.6, y: start.y },
        { x: start.x + STUB * 1.6, y: end.y - apart },
        { x: end.x, y: end.y - apart },
      ],
    }
  }

  const sides = facing(from, to)
  const start = anchorOf(from, relation.fromColumns[0], sides.from)
  const end = anchorOf(to, relation.toColumns[0], sides.to)
  return { start, end, points: elbow(start, end) }
}

const FOOT = 11
const SPREAD = 5

function CrowsFoot({ at, lit }: { at: Anchor; lit: boolean }) {
  const base = at.x + at.dir * FOOT
  const stroke = cn(lit ? 'stroke-accent' : 'stroke-kind-relation')

  return (
    <g className={stroke} strokeWidth={lit ? 2 : 1.25} strokeLinecap="round">
      <line x1={base} y1={at.y} x2={at.x} y2={at.y - SPREAD} />
      <line x1={base} y1={at.y} x2={at.x} y2={at.y + SPREAD} />
      <line x1={base} y1={at.y} x2={at.x} y2={at.y} />
    </g>
  )
}

function OneEnd({ at, optional, lit }: { at: Anchor; optional: boolean; lit: boolean }) {
  const bar = at.x + at.dir * 9
  const stroke = cn(lit ? 'stroke-accent' : 'stroke-kind-relation')

  return (
    <g className={stroke} strokeWidth={lit ? 2 : 1.25} strokeLinecap="round">
      <line x1={bar} y1={at.y - SPREAD} x2={bar} y2={at.y + SPREAD} />
      {optional ? (
        <circle cx={at.x + at.dir * 16} cy={at.y} r={3.2} className="fill-canvas" />
      ) : null}
    </g>
  )
}

/** The constraint's name, halfway along the segment that runs between the two boxes. */
function Label({ relation, points }: { relation: Relation; points: Point[] }) {
  const one = points[1]
  const other = points[2]
  if (!one || !other) return null

  const font = canvasFont(FIELD_SIZE - 1)
  const text = ellipsed(relation.name, font, 170)
  const width = Math.min(174, text.length * 6 + 10)

  return (
    <g pointerEvents="none">
      <rect
        x={(one.x + other.x) / 2 - width / 2}
        y={(one.y + other.y) / 2 - 7}
        width={width}
        height={14}
        rx={3}
        className="fill-canvas"
        opacity={0.85}
      />
      <text
        x={(one.x + other.x) / 2}
        y={(one.y + other.y) / 2}
        textAnchor="middle"
        dominantBaseline="central"
        fontSize={FIELD_SIZE - 1}
        className="fill-faint"
        style={{ fontFamily: 'var(--font-mono)' }}
      >
        {text}
      </text>
    </g>
  )
}
