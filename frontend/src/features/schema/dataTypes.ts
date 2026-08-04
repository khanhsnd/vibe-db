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

/**
 * The BSON types, which is what a MongoDB field is declared as wherever one is declared at all:
 * the `bsonType` of a collection's validator, and the `to` of the `$convert` that changes a field's
 * type across its documents. They are the server's own spellings — `objectId`, not `ObjectId` —
 * because they reach the command verbatim, exactly as a SQL type does.
 */
const MONGO = [
  'int',
  'long',
  'double',
  'decimal',
  'bool',
  'string',
  'objectId',
  'date',
  'timestamp',
  'object',
  'array',
  'binData',
  'regex',
  'javascript',
  'null',
  'minKey',
  'maxKey',
] as const

// The access methods an index is usually built with. Like the type list these are suggestions:
// the cell still takes anything typed into it, because a server may know methods of its own.
const MYSQL_INDEX_METHODS = ['BTREE', 'HASH'] as const
const POSTGRES_INDEX_METHODS = ['btree', 'hash', 'gin', 'gist', 'brin', 'spgist'] as const

/**
 * MongoDB keeps in an index's *key* what SQL puts in a USING clause, so these are the values a key
 * may take rather than a clause of their own. The blank one is the ordinary ascending index, which
 * is what an empty method already means everywhere else.
 */
const MONGO_INDEX_METHODS = ['', 'hashed', 'text', '2dsphere', '2d'] as const

/** What a foreign key does to the rows it holds when the row it points at moves or goes. */
export const REFERENTIAL_ACTIONS = [
  'NO ACTION',
  'RESTRICT',
  'CASCADE',
  'SET NULL',
  'SET DEFAULT',
] as const

export function dataTypes(engine: string): readonly string[] {
  switch (engineInfo(engine).value) {
    case 'postgres':
      return POSTGRES
    case 'mongo':
      return MONGO
    default:
      return MYSQL
  }
}

export function indexMethods(engine: string): readonly string[] {
  switch (engineInfo(engine).value) {
    case 'postgres':
      return POSTGRES_INDEX_METHODS
    case 'mongo':
      return MONGO_INDEX_METHODS
    default:
      return MYSQL_INDEX_METHODS
  }
}
