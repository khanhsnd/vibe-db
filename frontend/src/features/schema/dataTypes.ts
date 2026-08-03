import { engineInfo } from '@/bridge/connections'

/**
 * The types an engine is usually given, in the order a designer offers them: numbers, then text,
 * then bytes, then time, then the rest. Suggestions and not a rulebook — the Type cell still takes
 * anything typed into it, because a server also takes domains, enums and types this list has never
 * heard of.
 */
const MYSQL = [
  'int',
  'bigint',
  'smallint',
  'tinyint',
  'mediumint',
  'decimal',
  'float',
  'double',
  'bit',
  'boolean',
  'varchar',
  'char',
  'text',
  'tinytext',
  'mediumtext',
  'longtext',
  'enum',
  'set',
  'json',
  'binary',
  'varbinary',
  'blob',
  'tinyblob',
  'mediumblob',
  'longblob',
  'date',
  'datetime',
  'timestamp',
  'time',
  'year',
] as const

const POSTGRES = [
  'integer',
  'bigint',
  'smallint',
  'serial',
  'bigserial',
  'smallserial',
  'numeric',
  'real',
  'double precision',
  'money',
  'boolean',
  'varchar',
  'char',
  'text',
  'uuid',
  'json',
  'jsonb',
  'xml',
  'bytea',
  'date',
  'timestamp',
  'timestamptz',
  'time',
  'timetz',
  'interval',
  'inet',
  'cidr',
  'macaddr',
  'bit',
  'varbit',
] as const

export function dataTypes(engine: string): readonly string[] {
  return engineInfo(engine).value === 'postgres' ? POSTGRES : MYSQL
}
