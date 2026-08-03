/**
 * A declared type split the way the designer shows it: a base name plus the length and scale
 * in its parentheses. `varchar(255)` is `{ base: 'varchar', length: 255, scale: null }`.
 */
export interface FieldType {
  base: string
  length: number | null
  scale: number | null
}

// A type carrying anything after its parentheses — `timestamp(6) with time zone` — is kept
// whole as the base instead, because putting the parentheses back would move them.
const PARAMETERISED = /^([^(]+)\(\s*(\d+)\s*(?:,\s*(\d+)\s*)?\)\s*$/

export function parseFieldType(text: string): FieldType {
  const found = PARAMETERISED.exec(text.trim())
  if (!found) return { base: text.trim(), length: null, scale: null }

  const [, base, length, scale] = found
  return {
    base: (base ?? '').trim(),
    length: Number(length),
    scale: scale === undefined ? null : Number(scale),
  }
}

export function formatFieldType(type: FieldType): string {
  const base = type.base.trim()
  if (base === '' || type.length === null) return base
  if (type.scale === null) return `${base}(${type.length})`
  return `${base}(${type.length},${type.scale})`
}
