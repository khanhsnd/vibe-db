import {
  Binary,
  Braces,
  Calendar,
  CaseSensitive,
  Hash,
  ToggleLeft,
  Type,
  type LucideIcon,
} from 'lucide-react'

import { cn } from '@/lib/cn'

export type TypeKind = 'bool' | 'temporal' | 'json' | 'binary' | 'numeric' | 'text' | 'other'

/** Read in order, so `INTERVAL` is temporal before `INT` can claim it. */
const KINDS: ReadonlyArray<[TypeKind, RegExp]> = [
  ['bool', /^(BOOL|TINYINT\(1\))/i],
  ['temporal', /^(DATE|TIME|TIMESTAMP|INTERVAL)/i],
  ['json', /^JSON/i],
  ['binary', /^(BLOB|BYTEA|BINARY|VARBINARY|TINYBLOB|MEDIUMBLOB|LONGBLOB)/i],
  [
    'numeric',
    /^(TINYINT|SMALLINT|MEDIUMINT|INT|BIGINT|SMALLSERIAL|BIGSERIAL|SERIAL|DECIMAL|NUMERIC|REAL|FLOAT|DOUBLE|MONEY|BIT|YEAR|OID)/i,
  ],
  ['text', /^(CHAR|VARCHAR|TEXT|TINYTEXT|MEDIUMTEXT|LONGTEXT|ENUM|SET|NAME|CITEXT|UUID|XML)/i],
]

const ICONS: Record<TypeKind, LucideIcon> = {
  bool: ToggleLeft,
  temporal: Calendar,
  json: Braces,
  binary: Binary,
  numeric: Hash,
  text: CaseSensitive,
  other: Type,
}

/**
 * The colour each class is drawn in, so a wide header is read as a shape and a colour rather than
 * by spelling out seven type names. `other` and `binary` share the muted one deliberately: neither
 * is a class the reader can do anything different with.
 */
const COLOURS: Record<TypeKind, string> = {
  bool: 'text-type-bool',
  temporal: 'text-type-temporal',
  json: 'text-type-json',
  binary: 'text-type-binary',
  numeric: 'text-type-numeric',
  text: 'text-type-text',
  other: 'text-type-other',
}

export function typeKind(type: string): TypeKind {
  for (const [kind, pattern] of KINDS) {
    if (pattern.test(type)) return kind
  }
  return 'other'
}

export function isNumeric(type: string): boolean {
  return typeKind(type) === 'numeric'
}

/** The control a temporal column opens, named as the `<input type>` that draws it. */
export type TemporalPicker = 'date' | 'time' | 'datetime-local'

/** Read in order, so `TIMESTAMP` is a date and a time before `TIME` can claim it. */
const PICKERS: ReadonlyArray<[TemporalPicker, RegExp]> = [
  ['datetime-local', /^(DATETIME|TIMESTAMP)/i],
  ['date', /^DATE/i],
  ['time', /^TIME/i],
]

/**
 * Which picker a column's cells open, or nothing where the type has no point in time to draw —
 * an `INTERVAL` is temporal but is a length of time rather than a moment.
 */
export function temporalPicker(type: string): TemporalPicker | null {
  if (typeKind(type) !== 'temporal') return null
  return PICKERS.find(([, pattern]) => pattern.test(type))?.[0] ?? null
}

export function ColumnTypeIcon({ type }: { type: string }) {
  const kind = typeKind(type)
  const Icon = ICONS[kind]
  return <Icon size={12} className={cn('shrink-0', COLOURS[kind])} />
}
