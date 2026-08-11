import { colourInfo } from '@/bridge/connections'

import type { DiagramShape } from './model'
import { canvasFont, ellipsed, NOTE_SIZE, wrapped } from './text'

/**
 * One thing the reader drew beside the tables — a note, a box round a group of them, an ellipse, a
 * line of text. None of it means anything to a server: it is what the picture is *for*, which is
 * saying why the tables are arranged the way they are.
 *
 * It is drawn under the boxes, always: a rectangle round four tables is a background, and one that
 * covered them would make the thing it was drawn to explain unreadable.
 */
interface DiagramShapeProps {
  shape: DiagramShape
  selected: boolean
  /** Whether its words are being typed right now, which is when the shape itself draws none. */
  editing: boolean
}

const PAD = 8
const LINE = 15

export function DiagramShapeView({ shape, selected, editing }: DiagramShapeProps) {
  const tint = colourInfo(shape.colour).colour || 'var(--fg-muted)'
  const font = canvasFont(NOTE_SIZE)
  const lines = editing ? [] : wrapped(shape.text, font, shape.width - PAD * 2)
  const room = Math.max(1, Math.floor((shape.height - PAD) / LINE))

  return (
    <g data-shape={shape.id} style={{ cursor: 'move' }}>
      {shape.kind === 'ellipse' ? (
        <ellipse
          cx={shape.x + shape.width / 2}
          cy={shape.y + shape.height / 2}
          rx={shape.width / 2}
          ry={shape.height / 2}
          style={{ fill: tint, fillOpacity: 0.08, stroke: tint }}
          strokeWidth={selected ? 2 : 1.25}
        />
      ) : shape.kind === 'text' ? (
        <rect
          x={shape.x}
          y={shape.y}
          width={shape.width}
          height={shape.height}
          fill="transparent"
          style={{ stroke: selected ? 'var(--accent)' : 'transparent' }}
          strokeDasharray="3 3"
        />
      ) : (
        <rect
          x={shape.x}
          y={shape.y}
          width={shape.width}
          height={shape.height}
          rx={shape.kind === 'note' ? 4 : 2}
          style={{
            fill: tint,
            fillOpacity: shape.kind === 'note' ? 0.16 : 0.06,
            stroke: tint,
          }}
          strokeWidth={selected ? 2 : 1.25}
          strokeDasharray={shape.kind === 'rectangle' ? '6 4' : undefined}
        />
      )}

      {selected && shape.kind !== 'text' ? (
        <rect
          x={shape.x - 2}
          y={shape.y - 2}
          width={shape.width + 4}
          height={shape.height + 4}
          rx={5}
          fill="none"
          className="stroke-accent"
          strokeWidth={1}
          strokeDasharray="4 3"
        />
      ) : null}

      {lines.slice(0, room).map((line, at) => (
        <text
          key={at}
          x={shape.x + PAD}
          y={shape.y + PAD + at * LINE + LINE / 2}
          dominantBaseline="central"
          fontSize={NOTE_SIZE}
          fontWeight={shape.kind === 'text' ? 600 : 400}
          style={{ fontFamily: 'var(--font-ui)', fill: shape.kind === 'text' ? tint : 'var(--fg)' }}
        >
          {/* The last line that fits says there is more, rather than the rest vanishing silently. */}
          {at === room - 1 && lines.length > room
            ? ellipsed(`${line} …`, canvasFont(NOTE_SIZE), shape.width - PAD * 2)
            : line}
        </text>
      ))}

      <rect
        data-shape-resize={shape.id}
        x={shape.x + shape.width - 6}
        y={shape.y + shape.height - 6}
        width={12}
        height={12}
        fill="transparent"
        style={{ cursor: 'nwse-resize' }}
      />
      {selected ? (
        <rect
          x={shape.x + shape.width - 4}
          y={shape.y + shape.height - 4}
          width={7}
          height={7}
          className="fill-accent"
          pointerEvents="none"
        />
      ) : null}
    </g>
  )
}
