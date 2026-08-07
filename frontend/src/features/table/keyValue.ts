/**
 * What a value of one type *is*, which is the whole of what the panel under a key space needs to know
 * about it. The notation is the driver's and is not reinterpreted here: a string is itself, the five
 * containers are the JSON `internal/driver/redis/value.go` wrote, and what goes back is what
 * `parseValue` reads. There is **no second spelling of a value anywhere in the app** — which is also
 * why the panel edits it as text and not as a table of fields.
 */
export type ValueShape = 'fields' | 'scores' | 'string' | 'json' | 'opaque'

export function shapeOf(type: string): ValueShape {
  switch (type) {
    // The two values whose members are **pairs the reader thinks in** — a field and what it holds, an
    // element and its rank — are laid out as those two columns rather than as the object they cross
    // the wire as. They differ only in what the columns are called, the shape being the same JSON;
    // see `readMembers`. A list and a set are positions rather than pairs, so they stay text.
    case 'hash':
      return 'fields'
    case 'zset':
      return 'scores'
    case 'string':
      return 'string'
    case 'list':
    case 'set':
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
 * How the driver spells that mark, exactly: `… 50 of 4321` at the very end — see `redis.sampled`. It
 * is matched anchored rather than by looking for the character anywhere, because a hash field may
 * legitimately hold one and cutting there would cut the value in half.
 */
const SAMPLE_TAIL = /\s*…\s*(\d+) of (\d+)\s*$/

/**
 * The value with that mark taken off, which is what a sample is *read* from. A sampled container is
 * still the fifty members it showed, and drawing them as the fifty rows they are says far more than
 * refusing to lay them out at all — nothing here makes it writable, which is `refusedReason`'s.
 */
function withoutSample(text: string): string {
  return text.replace(SAMPLE_TAIL, '')
}

/** How many members the value really has, where it is a sample, and `null` where it is whole. */
export function sampledTotal(text: string): number | null {
  const found = SAMPLE_TAIL.exec(text)
  return found === null ? null : Number(found[2])
}

/**
 * Why this value cannot be written back, and `undefined` where it can. Both reasons are the driver's
 * own, said here before the reader types rather than after they press Apply.
 *
 * The mark is looked for **anywhere**, unlike `withoutSample` — because that is what the driver's own
 * refusal does, and a refusal this side that let something through would only fail at the server.
 */
export function refusedReason(type: string, value: string): string | undefined {
  if (value.includes(SAMPLE_MARK)) {
    return (
      'This value is a sample of a larger one, so it is read here and not written: the rest would ' +
      "go with the edit. The key's name and expiry can still be changed."
    )
  }
  if (shapeOf(type) === 'opaque') {
    return `A ${type} value is read here and not written: its shape is the server's own.`
  }
  return undefined
}

/** Whether this value is laid out as the pairs it holds rather than read whole as text. */
export function hasMembers(shape: ValueShape): boolean {
  return shape === 'fields' || shape === 'scores'
}

/** One member: a hash's field and what it holds, or a sorted set's element and its rank. */
export interface Member {
  field: string
  value: string
}

/**
 * The JSON of one of those two as the members it holds, or `null` for text this side cannot read as
 * one — which is what sends the panel back to plain text, so a reader retyping a value into a
 * different shape is never left without a box to type in.
 *
 * The **order** is the order the text spells it in. `JSON.parse` alone would not keep it: JavaScript
 * puts integer-like property names first and in numeric order, so a hash whose fields are `2` and
 * `10` would come back rearranged and go back rearranged.
 */
export function readMembers(text: string): Member[] | null {
  const whole = withoutSample(text)

  let held: unknown
  try {
    held = JSON.parse(whole)
  } catch {
    return null
  }
  if (held === null || typeof held !== 'object' || Array.isArray(held)) return null

  return Object.entries(held)
    .map(([field, value]) => ({ field, value: String(value) }))
    .sort((a, b) => spelledAt(whole, a.field) - spelledAt(whole, b.field))
}

/** Where a field is spelled in the JSON it came from, which is the order it was written in. */
function spelledAt(text: string, field: string): number {
  const at = text.indexOf(`${JSON.stringify(field)}:`)
  return at < 0 ? Number.MAX_SAFE_INTEGER : at
}

/**
 * The members back as the JSON the driver parses. Written by hand for the reason the driver writes
 * one by hand: **order**, which `JSON.stringify` over a map would lose again.
 */
export function writeMembers(members: readonly Member[]): string {
  const pairs = members.map(
    (member) => `${JSON.stringify(member.field)}:${JSON.stringify(member.value)}`,
  )
  return `{${pairs.join(',')}}`
}

/** What an added member is called, so two adds in a row do not make one of them twice. */
export function freshField(shape: ValueShape, members: readonly Member[]): string {
  const stem = shape === 'scores' ? 'element' : 'field'
  const taken = new Set(members.map((member) => member.field))

  for (let n = 1; ; n += 1) {
    const name = `${stem}${n}`
    if (!taken.has(name)) return name
  }
}

/**
 * What an added member holds before the reader types into it. A sorted set's is `0` and not blank:
 * a rank is a number, and the driver refuses a member whose score is not one — so an empty box would
 * be a row that could only fail.
 */
export function freshValue(shape: ValueShape): string {
  return shape === 'scores' ? '0' : ''
}
