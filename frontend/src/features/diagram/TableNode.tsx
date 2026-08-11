import { ChevronDown, ChevronRight, KeyRound, Spline, Table2 } from 'lucide-react'

import { colourInfo } from '@/bridge/connections'
import { TYPE_FILL, typeKind } from '@/features/results/columnType'
import { cn } from '@/lib/cn'

import { HEADER_HEIGHT, ROW_HEIGHT, roundedTop, type Box } from './model'
import { canvasFont, ellipsed, FIELD_SIZE, fieldType, TITLE_SIZE } from './text'

/**
 * One table on the canvas.
 *
 * It draws and nothing else: every press on it is read by the canvas off the `data-` attributes
 * below, so there is one place that knows what a drag means and this file only has to be right about
 * where things are. The measurements it lays out by are `model.ts`'s, which is what the lines
 * between the boxes are anchored with — the two would drift apart if either kept its own.
 */
interface TableNodeProps {
  box: Box
  selected: boolean
  /** Whether a relation under the pointer or in the selection touches this table. */
  related: boolean
  /** The columns that relation lands on, which are the rows worth marking. */
  marked: ReadonlySet<string>
  /** Whether a line may be drawn from this box at all — see `EngineInfo.hasForeignKeys`. */
  linkable: boolean
  /** What this engine calls a table, for the count under an unread box. */
  word: string
}

export function TableNode({ box, selected, related, marked, linkable, word }: TableNodeProps) {
  const { node, width, height } = box
  const tint = colourInfo(node.colour).colour
  const title = canvasFont(TITLE_SIZE, 600)
  const field = canvasFont(FIELD_SIZE)

  const count = box.known ? String(box.total) : ''
  const countWidth = count === '' ? 0 : 22
  const name = ellipsed(node.table, title, width - 36 - countWidth - 6)

  return (
    <g className="group" data-node={node.table} style={{ cursor: 'move' }}>
      <rect
        x={node.x}
        y={node.y}
        width={width}
        height={height}
        rx={5}
        className={cn(
          'fill-panel',
          selected ? 'stroke-accent' : related ? 'stroke-kind-relation' : 'stroke-line-strong',
        )}
        strokeWidth={selected ? 2 : 1}
      />

      {/* The header, and the connection-style colour the reader marked this box with over it: a
          wash rather than a fill, so the name is still read against the same shade everywhere. */}
      <path d={roundedTop(node.x, node.y, width, HEADER_HEIGHT)} className="fill-bar" />
      {tint === '' ? null : (
        <path
          d={roundedTop(node.x, node.y, width, HEADER_HEIGHT)}
          style={{ fill: tint, opacity: 0.18 }}
        />
      )}
      <line
        x1={node.x}
        y1={node.y + HEADER_HEIGHT}
        x2={node.x + width}
        y2={node.y + HEADER_HEIGHT}
        className="stroke-line"
      />

      {/* Folding the box shut is a press on the caret alone, exactly as it is in the tree. */}
      <g data-collapse={node.table} style={{ cursor: 'pointer' }}>
        <rect
          x={node.x + 2}
          y={node.y + 4}
          width={16}
          height={16}
          fill="transparent"
        />
        {node.collapsed ? (
          <ChevronRight x={node.x + 4} y={node.y + 6} size={12} className="text-faint" />
        ) : (
          <ChevronDown x={node.x + 4} y={node.y + 6} size={12} className="text-faint" />
        )}
      </g>

      <Table2 x={node.x + 20} y={node.y + 6} size={12} className="text-kind-table" />
      <text
        x={node.x + 36}
        y={node.y + HEADER_HEIGHT / 2}
        dominantBaseline="central"
        fontSize={TITLE_SIZE}
        fontWeight={600}
        className="fill-fg"
        style={{ fontFamily: 'var(--font-ui)' }}
      >
        {name}
      </text>
      {count === '' ? null : (
        <text
          x={node.x + width - 8}
          y={node.y + HEADER_HEIGHT / 2}
          textAnchor="end"
          dominantBaseline="central"
          fontSize={FIELD_SIZE - 1}
          className="fill-faint"
          style={{ fontFamily: 'var(--font-mono)' }}
        >
          {count}
        </text>
      )}

      {box.fields.map((column, at) => {
        const top = node.y + HEADER_HEIGHT + at * ROW_HEIGHT
        const middle = top + ROW_HEIGHT / 2
        const type = ellipsed(fieldType(column), field, width / 2)
        const typeWidth = type === '' ? 0 : Math.ceil(width / 2)
        const primary = box.primary.has(column.name)
        const foreign = box.foreign.has(column.name)

        return (
          <g key={column.name} data-field={column.name}>
            {marked.has(column.name) ? (
              <rect
                x={node.x + 1}
                y={top}
                width={width - 2}
                height={ROW_HEIGHT}
                className="fill-kind-relation"
                opacity={0.14}
              />
            ) : null}
            <rect x={node.x} y={top} width={width} height={ROW_HEIGHT} fill="transparent" />

            {primary ? (
              <KeyRound
                x={node.x + 6}
                y={middle - 5}
                size={10}
                style={{ color: 'var(--grid-key)' }}
              />
            ) : foreign ? (
              <Spline x={node.x + 6} y={middle - 5} size={10} className="text-kind-relation" />
            ) : null}

            <text
              x={node.x + 20}
              y={middle}
              dominantBaseline="central"
              fontSize={FIELD_SIZE}
              className={primary ? 'fill-fg' : 'fill-muted'}
              style={{ fontFamily: 'var(--font-ui)' }}
            >
              {ellipsed(column.name, field, width - 28 - typeWidth)}
            </text>

            <text
              x={node.x + width - 8}
              y={middle}
              textAnchor="end"
              dominantBaseline="central"
              fontSize={FIELD_SIZE - 1}
              className={TYPE_FILL[typeKind(fieldType(column))]}
              opacity={0.85}
              style={{ fontFamily: 'var(--font-mono)' }}
            >
              {type}
            </text>

          </g>
        )
      })}

      {/* A box whose shape has not been read yet says so, rather than reading as a table with no
          fields at all. */}
      {!box.known && !node.collapsed ? (
        <text
          x={node.x + width / 2}
          y={node.y + HEADER_HEIGHT + 12}
          textAnchor="middle"
          dominantBaseline="central"
          fontSize={FIELD_SIZE}
          className="fill-faint"
          style={{ fontFamily: 'var(--font-ui)' }}
        >
          {`Reading the ${word.toLowerCase()}…`}
        </text>
      ) : null}

      {/* The right edge is the width, so it is what the reader drags to change it. */}
      <rect
        data-resize={node.table}
        x={node.x + width - 3}
        y={node.y}
        width={6}
        height={height}
        fill="transparent"
        style={{ cursor: 'ew-resize' }}
      />

      {/* Where a relation is dragged from — **over** the resize edge, since they sit on the same
          pixels and a line is what a reader came to the edge of a field row for. They appear on the
          box the pointer is over, so a canvas at rest is boxes and lines and nothing else. */}
      {linkable
        ? box.fields.flatMap((column, at) =>
            ([-1, 1] as const).map((dir) => (
              <circle
                key={`${column.name}:${dir}`}
                data-port={dir}
                data-field={column.name}
                cx={dir > 0 ? node.x + width : node.x}
                cy={node.y + HEADER_HEIGHT + at * ROW_HEIGHT + ROW_HEIGHT / 2}
                r={4}
                className="fill-canvas stroke-kind-relation opacity-0 group-hover:opacity-100"
                strokeWidth={1.5}
                style={{ cursor: 'crosshair' }}
              />
            )),
          )
        : null}
    </g>
  )
}
