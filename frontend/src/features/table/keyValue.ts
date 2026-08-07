/**
 * What a value of one type *is*, which is the whole of what the panel under a key space needs to know
 * about it. The notation is the driver's and is not reinterpreted here: a string is itself, the five
 * containers are the JSON `internal/driver/redis/value.go` wrote, and what goes back is what
 * `parseValue` reads. There is **no second spelling of a value anywhere in the app** — which is also
 * why the panel edits it as text and not as a table of fields.
 */
export type ValueShape = 'string' | 'json' | 'opaque'

export function shapeOf(type: string): ValueShape {
  switch (type) {
    case 'string':
      return 'string'
    case 'list':
    case 'set':
    case 'hash':
    case 'zset':
      return 'json'
    // A stream is the server's own ids and is append-only, and a module's type has no general shape
    // at all. Both are read and neither is written — the driver refuses either, and the panel says so
    // rather than offering an edit that could only fail.
    default:
      return 'opaque'
  }
}

/**
 * What a value the reader is only being shown part of ends with — `redis.sampleMark`. Writing such a
 * value back would drop whatever the grid did not show, so the driver refuses it; the panel reads the
 * same mark to say why *before* the reader has typed into it.
 */
const SAMPLE_MARK = '…'

/**
 * Why this value cannot be written back, and `undefined` where it can. Both reasons are the driver's
 * own, said here before the reader types rather than after they press Apply.
 */
export function refusedReason(type: string, value: string): string | undefined {
  if (value.includes(SAMPLE_MARK)) {
    return 'This value is a sample of a larger one. Writing it back would drop whatever the grid did not show.'
  }
  if (shapeOf(type) === 'opaque') {
    return `A ${type} value is read here and not written: its shape is the server's own.`
  }
  return undefined
}
