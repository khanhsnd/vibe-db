/**
 * What a cell actually *holds*, read off the value rather than off its column's declared type.
 *
 * The type is not enough on its own and never was: a `TEXT` column holds JSON as often as a `JSON`
 * one does, a `JSON` column holds GeoJSON, and every value arrives here as a string whatever it
 * started as. So the value is what decides, and the type is only a hint where the value is
 * ambiguous. Nothing here parses SQL — it reads one cell.
 */

/** The bytes marker the driver writes for a value that is not text — `sqlkit.BinaryPrefix`. */
const BINARY = /^\\x([0-9a-f]{2})*$/i

export type ValueKind =
  | 'null'
  | 'empty'
  | 'binary'
  | 'geojson'
  | 'json'
  | 'document'
  | 'array'
  | 'text'

/**
 * The types a document store gives a field that holds a document or an array — `mongo.typeNames`,
 * and the only engine that says either. A value under one of those is written in **that engine's own
 * notation**: bare keys, `ObjectId(…)`, `ISODate(…)`. It is not JSON and `{ a: 1 }` will never parse
 * as any, which is why a document used to reach the viewer with no view but `Raw`.
 *
 * **Laying one out is the driver's** — how text is laid out is lexical and therefore the engine's,
 * so it arrives through `ValueViewer.formatValue` and nothing here writes a second formatter.
 */
const DOCUMENT_TYPES = new Set(['object', 'array'])

export interface DetectedValue {
  kind: ValueKind
  /** How the reader hears it named, in the viewer's own tab strip and its status line. */
  label: string
}

/**
 * The GeoJSON object types. A JSON value is GeoJSON when its `type` is one of these *and* it
 * carries the member that type is defined by — a `{"type": "Point"}` with no coordinates is a
 * malformed shape, and drawing nothing for it says more than pretending it is a point at the
 * origin.
 */
const GEOMETRIES = new Set([
  'Point',
  'MultiPoint',
  'LineString',
  'MultiLineString',
  'Polygon',
  'MultiPolygon',
  'GeometryCollection',
])

export function detect(value: string | null, columnType = ''): DetectedValue {
  if (value === null) return { kind: 'null', label: 'NULL' }
  if (value === '') return { kind: 'empty', label: 'Empty' }

  if (BINARY.test(value)) {
    return { kind: 'binary', label: `${(value.length - 2) / 2} bytes` }
  }

  // Before JSON, so a document store's array of scalars — `[1, "two"]`, which happens to parse —
  // still reads in the notation the grid drew it in and its own parser reads back.
  if (DOCUMENT_TYPES.has(columnType)) return { kind: 'document', label: columnType }

  const parsed = parseJSON(value)
  if (parsed !== undefined) {
    if (isGeoJSON(parsed)) return { kind: 'geojson', label: geoLabel(parsed) }
    if (Array.isArray(parsed)) return { kind: 'json', label: `Array of ${parsed.length}` }
    if (parsed !== null && typeof parsed === 'object') {
      return { kind: 'json', label: `Object of ${Object.keys(parsed).length}` }
    }
  }

  // A PostgreSQL array comes back in its own literal form — `{a,b,c}` — which is not JSON and
  // never will be. It is read as an array only where the column says it is one, because `{a,b}`
  // is also perfectly good text.
  if (columnType.endsWith('[]') || columnType.startsWith('_')) {
    const items = parseArray(value)
    if (items) return { kind: 'array', label: `Array of ${items.length}` }
  }

  return { kind: 'text', label: `${value.length} characters` }
}

function parseJSON(value: string): unknown {
  const trimmed = value.trimStart()
  // Only a `{` or a `[` is worth trying. Without this every number and every date in the grid is
  // handed to JSON.parse, which happily accepts `42` and calls it a document.
  if (!trimmed.startsWith('{') && !trimmed.startsWith('[')) return undefined

  try {
    return JSON.parse(value) as unknown
  } catch {
    return undefined
  }
}

function isGeoJSON(parsed: unknown): parsed is GeoJSON {
  if (parsed === null || typeof parsed !== 'object' || Array.isArray(parsed)) return false

  const value = parsed as Record<string, unknown>
  if (typeof value.type !== 'string') return false

  if (value.type === 'FeatureCollection') return Array.isArray(value.features)
  if (value.type === 'Feature') return 'geometry' in value
  if (value.type === 'GeometryCollection') return Array.isArray(value.geometries)
  return GEOMETRIES.has(value.type) && Array.isArray(value.coordinates)
}

/** Only as much of GeoJSON as drawing it needs. The rest of a feature is read in the JSON tab. */
export interface GeoJSON {
  type: string
  coordinates?: unknown
  geometry?: GeoJSON | null
  geometries?: GeoJSON[]
  features?: Array<{ geometry?: GeoJSON | null }>
}

function geoLabel(shape: GeoJSON): string {
  if (shape.type === 'FeatureCollection') return `${shape.features?.length ?? 0} features`
  return shape.type
}

/**
 * A PostgreSQL array literal, one level deep. `{a,"b,c",NULL}` is three items: the quoting is the
 * server's, `NULL` unquoted is the absence of a value, and a nested `{…}` is kept whole rather
 * than flattened — a two-dimensional array is a list of lists and reads as one.
 */
export function parseArray(value: string): string[] | null {
  if (!value.startsWith('{') || !value.endsWith('}')) return null

  const items: string[] = []
  let item = ''
  let quoted = false
  let depth = 0

  for (let i = 1; i < value.length - 1; i++) {
    const c = value[i]

    if (quoted) {
      if (c === '\\') {
        item += value[++i] ?? ''
        continue
      }
      if (c === '"') {
        quoted = false
        continue
      }
      item += c
      continue
    }

    if (c === '"') {
      quoted = true
      continue
    }
    if (c === '{') depth++
    if (c === '}') depth--
    if (c === ',' && depth === 0) {
      items.push(item)
      item = ''
      continue
    }
    item += c
  }

  // A trailing item, and the one case where there is none: `{}` is the empty array and not an
  // array of one empty string.
  if (item !== '' || items.length > 0) items.push(item)
  return items
}

/** The bytes behind a `\x…` value. */
export function decodeBinary(value: string): Uint8Array {
  const hex = value.slice(2)
  const bytes = new Uint8Array(hex.length / 2)
  for (let i = 0; i < bytes.length; i++) {
    bytes[i] = Number.parseInt(hex.slice(i * 2, i * 2 + 2), 16)
  }
  return bytes
}

/**
 * The image a blob holds, named by its first bytes rather than by anything the column says —
 * a `BLOB` column carries no type, and its first bytes are the only honest evidence there is.
 */
const SIGNATURES: ReadonlyArray<{ media: string; magic: number[] }> = [
  { media: 'image/png', magic: [0x89, 0x50, 0x4e, 0x47] },
  { media: 'image/jpeg', magic: [0xff, 0xd8, 0xff] },
  { media: 'image/gif', magic: [0x47, 0x49, 0x46, 0x38] },
  { media: 'image/bmp', magic: [0x42, 0x4d] },
  { media: 'image/webp', magic: [0x52, 0x49, 0x46, 0x46] },
]

export function imageType(bytes: Uint8Array): string | null {
  for (const { media, magic } of SIGNATURES) {
    if (magic.every((byte, i) => bytes[i] === byte)) return media
  }
  return null
}

/** Sixteen bytes to a line, offset, hex, then the printable characters — a hex dump reads so. */
export function hexDump(bytes: Uint8Array, limit: number): string[] {
  const lines: string[] = []

  for (let at = 0; at < Math.min(bytes.length, limit); at += 16) {
    const row = bytes.subarray(at, at + 16)
    const hex = [...row].map((byte) => byte.toString(16).padStart(2, '0'))
    const text = [...row]
      .map((byte) => (byte >= 0x20 && byte < 0x7f ? String.fromCharCode(byte) : '.'))
      .join('')

    lines.push(
      `${at.toString(16).padStart(8, '0')}  ` +
        `${hex.join(' ').padEnd(47, ' ')}  ` +
        `|${text}|`,
    )
  }
  return lines
}

/** JSON laid out, or the text back unchanged where it is not JSON after all. */
export function prettyJSON(value: string): string {
  const parsed = parseJSON(value)
  return parsed === undefined ? value : JSON.stringify(parsed, null, 2)
}
