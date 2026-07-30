import {
  completeFromList,
  ifNotIn,
  snippetCompletion,
  type Completion,
  type CompletionSource,
} from '@codemirror/autocomplete'
import { MySQL, PostgreSQL, sql, type SQLDialect, type SQLNamespace } from '@codemirror/lang-sql'
import type { EditorState, Extension } from '@codemirror/state'

import type { TableSchema } from '@/bridge/connections'

// `${…}` marks a field the caret jumps between with Tab. The label is what the user
// types to reach the snippet, so it stays lower-case even though the body is not.
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

function snippet(label: string, template: string): Completion {
  return snippetCompletion(template, {
    label,
    type: 'snippet',
    detail: firstLine(template),
  })
}

function firstLine(template: string): string {
  const line = template.split('\n')[0] ?? ''
  return line.replace(/\$\{(\w*)\}/g, '$1')
}

/**
 * How a name has to be written to resolve to the object it names. PostgreSQL folds an
 * unquoted identifier to lower case, so `AIFolders` reaches the server as `aifolders` and
 * a mixed-case table can only be read quoted. Completion inserts the form that will run,
 * which is the difference between a working statement and `UNKNOWN_TABLE`.
 *
 * MySQL folds nothing, so its names are offered as they are stored.
 */
function quoterFor(engine: string): (name: string) => string {
  if (engine !== 'postgres') return (name) => name

  return (name) => (/^[a-z_][a-z0-9_$]*$/.test(name) ? name : `"${name.replaceAll('"', '""')}"`)
}

// lang-sql keys its namespace by table name and resolves `FROM users u` aliases itself,
// so one flat map of the active namespace is the whole schema it needs.
function toNamespace(tables: TableSchema[], quote: (name: string) => string): SQLNamespace {
  return Object.fromEntries(
    tables.map((table) => [
      table.name,
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

function dialectFor(engine: string): SQLDialect {
  return engine === 'postgres' ? PostgreSQL : MySQL
}

// Any word in the statement that names a table counts, rather than only the one after
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

  // Both quotings are read whatever the engine, since only one of them can be an
  // identifier in a given statement and the other cannot name a table anyway.
  const found = new Map<string, TableSchema>()
  for (const [word] of statement.matchAll(/`[^`]+`|"[^"]+"|[\w$]+/g)) {
    const table = byName.get(word.replaceAll('`', '').replaceAll('"', '').toLowerCase())
    if (table) found.set(table.name, table)
  }
  return [...found.values()]
}

// Neither source belongs where the schema is already answering — after `alias.`, inside
// a string, inside a comment.
const OFF_LIMITS = [
  'CompositeIdentifier',
  'QuotedIdentifier',
  'String',
  'LineComment',
  'BlockComment',
]

export function sqlLanguage(tables: TableSchema[], engine: string): Extension {
  const quote = quoterFor(engine)
  const support = sql({
    dialect: dialectFor(engine),
    schema: toNamespace(tables, quote),
    upperCaseKeywords: true,
  })

  return [
    support,
    support.language.data.of({
      autocomplete: ifNotIn(OFF_LIMITS, completeFromList(SNIPPETS)),
    }),
    support.language.data.of({
      autocomplete: ifNotIn(OFF_LIMITS, bareColumnSource(tables, quote)),
    }),
  ]
}
