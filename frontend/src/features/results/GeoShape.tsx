import { useMemo } from 'react'

import type { GeoJSON } from './value'

const SIZE = { width: 460, height: 260 }
const PADDING = 12
/** A lone point has no extent to fit, so it is drawn at this radius and centred. */
const POINT_RADIUS = 3.5

type Point = [number, number]

/**
 * A GeoJSON value drawn as the shape it is. It is a **plain SVG** and deliberately not a map:
 * a basemap means tiles, tiles mean a network request, and this app has no business sending the
 * coordinates in a reader's database to a tile server to find out where they are. What the shape
 * *is* — a ring, a route, a scatter — is readable without one, and the numbers are in the JSON
 * tab beside it.
 *
 * Coordinates are fitted to the box rather than projected. At the size of one geometry the
 * difference is a slight vertical stretch, and a projection would imply a map that is not here.
 */
export function GeoShape({ shape }: { shape: GeoJSON }) {
  const drawn = useMemo(() => flatten(shape), [shape])
  const box = useMemo(() => bounds(drawn), [drawn])

  if (drawn.length === 0 || box === null) {
    return (
      <p className="p-3 text-xs text-muted">
        This shape carries no coordinates to draw.
      </p>
    )
  }

  const project = projector(box)

  return (
    <div className="flex h-full min-h-0 flex-col">
      <svg
        viewBox={`0 0 ${SIZE.width} ${SIZE.height}`}
        className="min-h-0 w-full flex-1"
        role="img"
        aria-label={`${shape.type} drawn from its coordinates`}
      >
        {drawn.map((part, index) =>
          part.kind === 'point' ? (
            <circle
              key={index}
              cx={project(part.at)[0]}
              cy={project(part.at)[1]}
              r={POINT_RADIUS}
              fill="var(--accent)"
            />
          ) : (
            <path
              key={index}
              d={path(part.points.map(project), part.kind === 'ring')}
              fill={part.kind === 'ring' ? 'var(--accent)' : 'none'}
              fillOpacity={part.kind === 'ring' ? 0.18 : undefined}
              stroke="var(--accent)"
              strokeWidth={1.5}
              strokeLinejoin="round"
            />
          ),
        )}
      </svg>

      <p className="shrink-0 border-t border-line px-3 py-1 font-mono text-2xs text-faint">
        {box[0].toFixed(4)}, {box[1].toFixed(4)} → {box[2].toFixed(4)}, {box[3].toFixed(4)}
      </p>
    </div>
  )
}

/** One thing to draw: a point, an open line, or a closed ring. */
type Part = { kind: 'point'; at: Point } | { kind: 'line' | 'ring'; points: Point[] }

/**
 * Every geometry a GeoJSON value holds, however deeply it is wrapped. A FeatureCollection of
 * MultiPolygons is a list of rings by the time it reaches the drawing above, which is the whole
 * point of doing it here: the SVG knows about points, lines and rings and nothing else.
 */
function flatten(shape: GeoJSON | null | undefined): Part[] {
  if (!shape || typeof shape.type !== 'string') return []

  switch (shape.type) {
    case 'FeatureCollection':
      return (shape.features ?? []).flatMap((feature) => flatten(feature.geometry))
    case 'Feature':
      return flatten(shape.geometry)
    case 'GeometryCollection':
      return (shape.geometries ?? []).flatMap(flatten)

    case 'Point':
      return asPoint(shape.coordinates) ? [{ kind: 'point', at: shape.coordinates as Point }] : []
    case 'MultiPoint':
      return asPoints(shape.coordinates).map((at) => ({ kind: 'point', at }))
    case 'LineString':
      return line(shape.coordinates, 'line')
    case 'MultiLineString':
      return asList(shape.coordinates).flatMap((one) => line(one, 'line'))
    case 'Polygon':
      return asList(shape.coordinates).flatMap((one) => line(one, 'ring'))
    case 'MultiPolygon':
      return asList(shape.coordinates).flatMap((polygon) =>
        asList(polygon).flatMap((one) => line(one, 'ring')),
      )
    default:
      return []
  }
}

function line(coordinates: unknown, kind: 'line' | 'ring'): Part[] {
  const points = asPoints(coordinates)
  return points.length > 1 ? [{ kind, points }] : []
}

function asPoint(value: unknown): value is Point {
  return (
    Array.isArray(value) &&
    value.length >= 2 &&
    typeof value[0] === 'number' &&
    typeof value[1] === 'number' &&
    Number.isFinite(value[0]) &&
    Number.isFinite(value[1])
  )
}

function asPoints(value: unknown): Point[] {
  return Array.isArray(value) ? value.filter(asPoint).map((one) => [one[0], one[1]] as Point) : []
}

function asList(value: unknown): unknown[] {
  return Array.isArray(value) ? value : []
}

/** `[minX, minY, maxX, maxY]`, or null where there is nothing with an extent. */
function bounds(parts: Part[]): [number, number, number, number] | null {
  const points = parts.flatMap((part) => (part.kind === 'point' ? [part.at] : part.points))
  if (points.length === 0) return null

  const xs = points.map(([x]) => x)
  const ys = points.map(([, y]) => y)
  return [Math.min(...xs), Math.min(...ys), Math.max(...xs), Math.max(...ys)]
}

/**
 * Coordinates into the box. `y` is flipped because latitude climbs northwards and an SVG's does
 * not, and a shape with no extent in one direction — a straight line, a single point — is centred
 * on that axis rather than divided by zero.
 */
function projector(box: [number, number, number, number]): (point: Point) => Point {
  const [minX, minY, maxX, maxY] = box
  const width = SIZE.width - PADDING * 2
  const height = SIZE.height - PADDING * 2

  const spanX = maxX - minX
  const spanY = maxY - minY

  return ([x, y]) => [
    spanX === 0 ? SIZE.width / 2 : PADDING + ((x - minX) / spanX) * width,
    spanY === 0 ? SIZE.height / 2 : PADDING + (1 - (y - minY) / spanY) * height,
  ]
}

function path(points: Point[], closed: boolean): string {
  const drawn = points.map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${x.toFixed(2)} ${y.toFixed(2)}`)
  return drawn.join(' ') + (closed ? ' Z' : '')
}
