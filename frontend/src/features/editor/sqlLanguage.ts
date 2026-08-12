import {
  completeFromList,
  ifNotIn,
  type Completion,
  type CompletionSource,
} from '@codemirror/autocomplete'
import {
  MSSQL,
  MySQL,
  PLSQL,
  PostgreSQL,
  SQLite,
  sql,
  type SQLDialect,
  type SQLNamespace,
} from '@codemirror/lang-sql'
import type { EditorState, Extension } from '@codemirror/state'

import type { TableSchema } from '@/bridge/connections'
import type { Snippet } from '@/bridge/snippets'

import { builtIn as snippet, saved } from './snippets'

/**
 * The tables one editor completes from. `tables` is the namespace it is pointed at, whose tables a
 * statement reaches by name alone; `qualified` is every sibling namespace of the same connection,
 * keyed by the name that qualifies a table living in it — a database on MySQL, a schema on
 * PostgreSQL. That key is the whole of what makes `orders o JOIN warehouse.stock s` complete on
 * both sides. Which namespaces are in it, and when they are read, is `useQuerySchema`'s.
 */
export interface QuerySchema {
  tables: TableSchema[]
  qualified: Record<string, TableSchema[]>
}

// The dozen this app ships with. The reader's own join them at the same word and sort above them —
// see `snippets.ts`, which is where `${…}` and the detail line are explained.
const SNIPPETS: readonly Completion[] = [
  snippet('select', 'SELECT ${columns}\nFROM ${table}\nWHERE ${condition}'),
  snippet('select all', 'SELECT *\nFROM ${table}\nLIMIT ${100}'),
  snippet('count', 'SELECT COUNT(*)\nFROM ${table}\nWHERE ${condition}'),
  snippet('insert', 'INSERT INTO ${table} (${columns})\nVALUES (${values})'),
  snippet('update', 'UPDATE ${table}\nSET ${column} = ${value}\nWHERE ${condition}'),
  snippet('delete', 'DELETE FROM ${table}\nWHERE ${condition}'),
  snippet('join', 'JOIN ${table} ON ${left} = ${right}'),
  snippet('left join', 'LEFT JOIN ${table} ON ${left} = ${right}'),
  snippet('group by', 'GROUP BY ${columns}\nHAVING ${condition}'),
  snippet('order by', 'ORDER BY ${column} DESC'),
  snippet(
    'with',
    'WITH ${name} AS (\n\tSELECT ${columns}\n\tFROM ${table}\n)\nSELECT *\nFROM ${name}',
  ),
  snippet('case', 'CASE WHEN ${condition} THEN ${value} ELSE ${fallback} END'),
]

/**
 * How a name has to be written to resolve to the object it names. Two engines fold an unquoted
 * identifier, and they fold it opposite ways: PostgreSQL lowers it, so `AIFolders` reaches the
 * server as `aifolders`; Oracle raises it, so `users` reaches it as `USERS`. Either way a name that
 * is not already in the engine's own case can only be read quoted, and completion inserts the form
 * that will run — which is the difference between a working statement and `UNKNOWN_TABLE`.
 *
 * MySQL, SQL Server and SQLite fold nothing, so their names are offered as they are stored.
 */
function quoterFor(engine: string): (name: string) => string {
  if (engine === 'postgres') return foldedBy(/^[a-z_][a-z0-9_$]*$/)
  if (engine === 'oracle') return foldedBy(/^[A-Z_][A-Z0-9_$#]*$/)
  return (name) => name
}

// A name matching the engine's own unquoted form is offered bare; anything else is quoted, the
// standard's own doubling escaping a quote inside it.
function foldedBy(plain: RegExp): (name: string) => string {
  return (name) => (plain.test(name) ? name : `"${name.replaceAll('"', '""')}"`)
}

/**
 * lang-sql resolves `FROM users u` aliases itself and nests a namespace by splitting its keys on
 * the dot, so naming a sibling namespace at the top is the whole of cross-namespace completion:
 * `warehouse.` offers its tables, `warehouse.stock.` offers their columns, and `FROM
 * warehouse.stock s … s.` resolves through both.
 */
function toNamespace(schema: QuerySchema, quote: (name: string) => string): SQLNamespace {
  const siblings = Object.entries(schema.qualified).map(([name, tables]) => [
    escapeDots(name),
    tablesOf(tables, quote),
  ])

  // A table of the editor's own namespace wins the bare name, because that is what an unqualified
  // name means; a sibling's table of the same name is still reached through its qualifier.
  return { ...Object.fromEntries(siblings), ...tablesOf(schema.tables, quote) }
}

function tablesOf(tables: TableSchema[], quote: (name: string) => string): SQLNamespace {
  return Object.fromEntries(
    tables.map((table) => [
      escapeDots(table.name),
      {
        self: {
          label: table.name,
          apply: quote(table.name),
          type: 'type',
          detail: `${table.columns.length} columns`,
        },
        children: table.columns.map((column) => ({
          label: column,
          apply: quote(column),
        })),
      },
    ]),
  )
}

// A dot in a key is how lang-sql is told to nest one level inside another, so a name that holds
// one of its own has to say that is what it is.
const escapeDots = (name: string) => name.replaceAll('.', '\\.')

// Every table the statement could be naming, written the way it would have to be written: a
// sibling namespace's table is only reachable qualified, and that is what the reader sees.
function reachable({ tables, qualified }: QuerySchema): TableSchema[] {
  return [
    ...tables,
    ...Object.entries(qualified).flatMap(([name, siblings]) =>
      siblings.map((table) => ({ name: `${name}.${table.name}`, columns: table.columns })),
    ),
  ]
}

// lang-sql only offers columns behind a `table.` prefix. Everywhere else in a statement
// — after SELECT, after WHERE, after ORDER BY — the columns of the tables that statement
// already names are what is being reached for, so they are offered bare.
function bareColumnSource(
  tables: TableSchema[],
  quote: (name: string) => string,
): CompletionSource {
  const byName = new Map(tables.map((table) => [table.name.toLowerCase(), table]))

  return (context) => {
    const word = context.matchBefore(/[\w$]*/)
    if (!word) return null

    const options = tablesInStatement(context.state, context.pos, byName).flatMap((table) =>
      table.columns.map((column) => ({
        label: column,
        apply: quote(column),
        type: 'property',
        detail: table.name,
        boost: 1,
      })),
    )
    if (options.length === 0) return null

    return { from: word.from, options, validFor: /^[\w$]*$/ }
  }
}

// One dialect per engine, which is what decides how the editor colours a name, a string and a
// comment: the bracket around `[dbo]` is an identifier on one engine and an array on another, and a
// `#` opens a comment on exactly one of the five.
const DIALECTS: Record<string, SQLDialect> = {
  postgres: PostgreSQL,
  sqlite: SQLite,
  sqlserver: MSSQL,
  oracle: PLSQL,
}

function dialectFor(engine: string): SQLDialect {
  return DIALECTS[engine] ?? MySQL
}

// A name is one word, or several joined by dots. Every quoting is read whatever the engine, since
// only one of them can be an identifier in a given statement and the others cannot name a table
// anyway.
const PART = String.raw`\`[^\`]+\`|"[^"]+"|\[[^\]]+\]|[\w$]+`
const PARTS = new RegExp(PART, 'g')
const PATH = new RegExp(String.raw`(?:${PART})(?:\s*\.\s*(?:${PART}))*`, 'g')

// Any name in the statement that names a table counts, rather than only the one after
// FROM. That reads aliased and joined tables for free, and it also catches the tables
// whose name is a reserved word, which the parser hands back as a keyword token.
function tablesInStatement(
  state: EditorState,
  pos: number,
  byName: Map<string, TableSchema>,
): TableSchema[] {
  const doc = state.doc.toString()
  const start = doc.lastIndexOf(';', pos - 1) + 1
  const end = doc.indexOf(';', pos)
  const statement = doc.slice(start, end === -1 ? doc.length : end)

  const found = new Map<string, TableSchema>()
  for (const [path] of statement.matchAll(PATH)) {
    const parts = (path.match(PARTS) ?? []).map(unquote)

    // A path is read longest first: `warehouse.stock.sku` names the table `warehouse.stock`, and
    // `stock.sku` names `stock`.
    for (let held = parts.length; held > 0; held--) {
      const table = byName.get(parts.slice(0, held).join('.').toLowerCase())
      if (table) {
        found.set(table.name, table)
        break
      }
    }
  }
  return [...found.values()]
}

const unquote = (part: string) => (/^[`"[]/.test(part) ? part.slice(1, -1) : part)

// Neither source belongs where the schema is already answering — after `alias.`, inside
// a string, inside a comment.
const OFF_LIMITS = [
  'CompositeIdentifier',
  'QuotedIdentifier',
  'String',
  'LineComment',
  'BlockComment',
]

export function sqlLanguage(
  schema: QuerySchema,
  engine: string,
  library: readonly Snippet[],
): Extension {
  const quote = quoterFor(engine)
  const support = sql({
    dialect: dialectFor(engine),
    schema: toNamespace(schema, quote),
    upperCaseKeywords: true,
  })

  return [
    support,
    support.language.data.of({
      autocomplete: ifNotIn(OFF_LIMITS, completeFromList([...SNIPPETS, ...saved(library)])),
    }),
    support.language.data.of({
      autocomplete: ifNotIn(OFF_LIMITS, bareColumnSource(reachable(schema), quote)),
    }),
  ]
}
