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

export function typeKind(type: string): TypeKind {
  for (const [kind, pattern] of KINDS) {
    if (pattern.test(type)) return kind
  }
  return 'other'
}

export function isNumeric(type: string): boolean {
  return typeKind(type) === 'numeric'
}

export function ColumnTypeIcon({ type }: { type: string }) {
  const Icon = ICONS[typeKind(type)]
  return <Icon size={12} className="shrink-0" />
}
