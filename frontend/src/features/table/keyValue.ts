/**
 * How a value of one type is laid out, which is what the panel under a key space edits it *as*.
 *
 * The notation itself is still the driver's: a container crosses the wire as the JSON
 * `internal/driver/redis/value.go` wrote and goes back as the JSON it parses. This file only reads
 * that JSON into the rows a reader edits and writes those rows back into it, which is why a field
 * table is not a second notation — nothing here invents a spelling the driver would refuse.
 */
export type ValueShape = 'string' | 'array' | 'pairs' | 'opaque'

export function shapeOf(type: string): ValueShape {
  switch (type) {
    case 'string':
      return 'string'
    case 'list':
    case 'set':
      return 'array'
    case 'hash':
    case 'zset':
      return 'pairs'
    // A stream is the server's own ids and is append-only, and a module's type has no general shape
    // at all. Both are read and neither is written — the driver refuses either, and the panel says
    // so rather than offering an edit that could only fail.
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

export function isSample(value: string): boolean {
  return value.includes(SAMPLE_MARK)
}

/** One member of a container: a field and its value on a hash or a sorted set, an index below one. */
export interface Member {
  field: string
  value: string
}

/**
 * A container's JSON as the members it holds, or `null` for one this side cannot read — which is what
 * the panel draws its reason from. A member's *order* is kept: a sorted set is its rank order, and
 * `JSON.parse` would hand back a hash whose fields are `2` and `10` in numeric order, JavaScript
 * having its own opinion about integer-like property names. So the pairs are put back into the order
 * the text spells them in.
 */
export function readMembers(shape: ValueShape, text: string): Member[] | null {
  try {
    switch (shape) {
      case 'array': {
        const held: unknown = JSON.parse(text)
        if (!Array.isArray(held)) return null
        return held.map((value, index) => ({ field: String(index), value: String(value) }))
      }
      case 'pairs': {
        const held: unknown = JSON.parse(text)
        if (held === null || typeof held !== 'object' || Array.isArray(held)) return null

        return Object.entries(held)
          .map(([field, value]) => ({ field, value: String(value) }))
          .sort((a, b) => spelledAt(text, a.field) - spelledAt(text, b.field))
      }
      default:
        return null
    }
  } catch {
    return null
  }
}

/** Where a field is spelled in the JSON it came from, which is the order it was written in. */
function spelledAt(text: string, field: string): number {
  const at = text.indexOf(`${JSON.stringify(field)}:`)
  return at < 0 ? Number.MAX_SAFE_INTEGER : at
}

/**
 * The members back as the JSON the driver parses. An object is written by hand for the reason the
 * driver writes one by hand: **order**, which `JSON.stringify` over a map would lose again.
 */
export function writeMembers(shape: ValueShape, members: readonly Member[]): string {
  if (shape === 'array') return JSON.stringify(members.map((member) => member.value))

  const pairs = members.map(
    (member) => `${JSON.stringify(member.field)}:${JSON.stringify(member.value)}`,
  )
  return `{${pairs.join(',')}}`
}

/** What an added member is called, so two adds in a row do not make one field twice. */
export function freshField(shape: ValueShape, members: readonly Member[]): string {
  if (shape === 'array') return String(members.length)

  const taken = new Set(members.map((member) => member.field))
  for (let n = 1; ; n += 1) {
    const name = `field${n}`
    if (!taken.has(name)) return name
  }
}

/** The indexes an array's fields carry, renumbered after one was taken out of the middle. */
export function renumbered(shape: ValueShape, members: readonly Member[]): Member[] {
  if (shape !== 'array') return [...members]
  return members.map((member, index) => ({ field: String(index), value: member.value }))
}
