import {
  completeFromList,
  ifNotIn,
  type Completion,
  type CompletionSource,
} from '@codemirror/autocomplete'
import { javascript } from '@codemirror/lang-javascript'
import { syntaxTree } from '@codemirror/language'
import { RangeSetBuilder, type Extension } from '@codemirror/state'
import {
  Decoration,
  EditorView,
  ViewPlugin,
  type DecorationSet,
  type ViewUpdate,
} from '@codemirror/view'

import type { TableSchema } from '@/bridge/connections'
import type { Snippet } from '@/bridge/snippets'

import { builtIn as snippet, saved } from './snippets'

/**
 * The editor for a document store. It is JavaScript to *highlight* — a command is `db.users.find({…})`
 * and its arguments are a document, so a brace is a brace and a string is a string — and it is
 * nothing like JavaScript to *run*: the driver reads one command and its arguments, and there are no
 * variables and no control flow. See `internal/driver/mongo/mql.go`, which is the grammar this
 * completes against.
 */

// The commands this app ships with. The reader's own join them at the same word and sort above them —
// see `snippets.ts`, which is where `${…}` and the detail line are explained.
const SNIPPETS: readonly Completion[] = [
  snippet('find', 'db.${collection}.find({ ${field}: ${value} })'),
  snippet('find all', 'db.${collection}.find({}).limit(${100})'),
  snippet('find one', 'db.${collection}.findOne({ _id: ObjectId("${id}") })'),
  snippet('count', 'db.${collection}.countDocuments({ ${field}: ${value} })'),
  snippet('distinct', 'db.${collection}.distinct("${field}")'),
  snippet('insert', 'db.${collection}.insertOne({ ${field}: ${value} })'),
  snippet('insert many', 'db.${collection}.insertMany([\n\t{ ${field}: ${value} }\n])'),
  snippet(
    'update',
    'db.${collection}.updateOne(\n\t{ ${field}: ${value} },\n\t{ $set: { ${field}: ${value} } }\n)',
  ),
  snippet(
    'update many',
    'db.${collection}.updateMany(\n\t{ ${field}: ${value} },\n\t{ $set: { ${field}: ${value} } }\n)',
  ),
  snippet('delete', 'db.${collection}.deleteOne({ _id: ObjectId("${id}") })'),
  snippet(
    'aggregate',
    'db.${collection}.aggregate([\n\t{ $match: { ${field}: ${value} } },\n' +
      '\t{ $group: { _id: "$${field}", count: { $sum: 1 } } },\n\t{ $sort: { count: -1 } }\n])',
  ),
  snippet('index', 'db.${collection}.createIndex({ ${field}: 1 }, { name: "${name}" })'),
  snippet('command', 'db.runCommand({ ${command}: 1 })'),
]

/**
 * The methods a collection answers to, and what each of them does. It is the driver's own list —
 * anything not here is refused by `onCollection` with a message saying so — which is why offering
 * something else would be worse than offering nothing.
 */
const COLLECTION_METHODS: ReadonlyArray<[string, string]> = [
  ['find', 'documents matching a filter'],
  ['findOne', 'the first document matching a filter'],
  ['aggregate', 'a pipeline over the collection'],
  ['countDocuments', 'how many documents match a filter'],
  ['estimatedDocumentCount', 'how many documents there are, from metadata'],
  ['distinct', 'the values one field holds'],
  ['insertOne', 'write one document'],
  ['insertMany', 'write several documents'],
  ['updateOne', 'change the first document matching a filter'],
  ['updateMany', 'change every document matching a filter'],
  ['replaceOne', 'replace the first document matching a filter'],
  ['deleteOne', 'remove the first document matching a filter'],
  ['deleteMany', 'remove every document matching a filter'],
  ['findOneAndUpdate', 'change one document and answer with it'],
  ['findOneAndReplace', 'replace one document and answer with it'],
  ['findOneAndDelete', 'remove one document and answer with it'],
  ['createIndex', 'build an index'],
  ['getIndexes', 'the indexes this collection has'],
  ['dropIndex', 'remove one index by name'],
  ['dropIndexes', 'remove every index but the one on _id'],
  ['stats', "this collection's storage"],
  ['drop', 'remove the collection and everything in it'],
]

/** The commands addressed to the database itself, which is what follows `db.` and no collection. */
const DATABASE_METHODS: ReadonlyArray<[string, string]> = [
  ['runCommand', 'run any command the server knows'],
  ['getCollectionNames', 'the collections this database holds'],
  ['aggregate', 'a pipeline over no collection at all'],
  ['createCollection', 'make a collection'],
  ['createView', 'make a view over a collection'],
  ['stats', "this database's storage"],
  ['dropDatabase', 'remove the database and every collection in it'],
  ['getCollection', 'a collection whose name is not a bare word'],
]

/** The cursor methods a read may be bounded by, which is what follows a closing bracket. */
const CURSOR_METHODS: ReadonlyArray<[string, string]> = [
  ['sort', 'order the documents'],
  ['skip', 'start further in'],
  ['limit', 'stop after so many'],
  ['count', 'how many there are rather than what they are'],
]

/**
 * The operators, which are what a document's keys mostly are: a query's, an update's and a
 * pipeline's, in one list. They are offered together rather than by position because where an
 * operator is legal is the server's rule and not one a completion list should pretend to know.
 */
const OPERATORS: readonly string[] = [
  '$eq', '$ne', '$gt', '$gte', '$lt', '$lte', '$in', '$nin',
  '$and', '$or', '$not', '$nor', '$exists', '$type', '$regex', '$options', '$expr', '$mod',
  '$all', '$elemMatch', '$size',
  '$set', '$unset', '$inc', '$mul', '$rename', '$min', '$max', '$currentDate', '$setOnInsert',
  '$push', '$pull', '$pullAll', '$addToSet', '$pop', '$each', '$position', '$slice',
  '$match', '$group', '$project', '$sort', '$limit', '$skip', '$unwind', '$lookup', '$count',
  '$facet', '$addFields', '$replaceRoot', '$sample', '$out', '$merge', '$sortByCount', '$bucket',
  '$sum', '$avg', '$first', '$last', '$push', '$stdDevPop', '$concat', '$toLower', '$toUpper',
  '$dateToString', '$cond', '$ifNull', '$switch', '$convert', '$toString', '$toInt', '$toDate',
]

/** The shell's own constructors — the one place a value that is not JSON is written. */
const CONSTRUCTORS: readonly string[] = [
  'ObjectId',
  'ISODate',
  'Date',
  'NumberLong',
  'NumberInt',
  'NumberDecimal',
  'NumberDouble',
  'UUID',
  'BinData',
  'Timestamp',
  'RegExp',
  'MinKey',
  'MaxKey',
]

/** `db.` and then what: every collection of the namespace, plus the database's own commands. */
function afterDb(collections: TableSchema[]): CompletionSource {
  const options: Completion[] = [
    ...collections.map((one) => ({
      label: one.name,
      type: 'type',
      detail: `${one.columns.length} fields`,
      boost: 1,
    })),
    ...methodOptions(DATABASE_METHODS),
  ]

  return (context) => {
    const before = context.matchBefore(/\bdb\.[\w$]*/)
    if (!before) return null
    return { from: before.from + 'db.'.length, options, validFor: /^[\w$]*$/ }
  }
}

/** `db.users.` and then what: the methods a collection answers to. */
const afterCollection: CompletionSource = (context) => {
  const before = context.matchBefore(/\bdb\.[\w$]+\.[\w$]*/)
  if (!before) return null

  const dot = before.text.lastIndexOf('.')
  return {
    from: before.from + dot + 1,
    options: methodOptions(COLLECTION_METHODS),
    validFor: /^[\w$]*$/,
  }
}

/** `).` and then what: the cursor methods, which is the only thing that can follow a call. */
const afterCall: CompletionSource = (context) => {
  const before = context.matchBefore(/\)\.[\w$]*/)
  if (!before) return null
  return {
    from: before.from + 2,
    options: methodOptions(CURSOR_METHODS),
    validFor: /^[\w$]*$/,
  }
}

/** A word beginning `$` is an operator, wherever in a document it sits. */
const operatorSource: CompletionSource = (context) => {
  const before = context.matchBefore(/\$[\w]*/)
  if (!before) return null

  return {
    from: before.from,
    options: [...new Set(OPERATORS)].map((name) => ({ label: name, type: 'keyword' })),
    validFor: /^\$[\w]*$/,
  }
}

/**
 * The fields of whichever collection the command names, offered bare. A document store declares no
 * fields, so these are the ones its documents were *found* to carry — which is the same list the
 * tree draws under the collection, and is a sample rather than a promise.
 */
function fieldSource(collections: TableSchema[]): CompletionSource {
  const byName = new Map(collections.map((one) => [one.name.toLowerCase(), one]))

  return (context) => {
    const word = context.matchBefore(/[\w$]*/)
    if (!word) return null

    const found = namedCollection(context.state.sliceDoc(0, context.pos), byName)
    if (!found) return null

    const options: Completion[] = [
      ...found.columns.map((field) => ({
        label: field,
        type: 'property',
        detail: found.name,
        boost: 1,
      })),
      ...CONSTRUCTORS.map((name) => ({ label: name, type: 'function' })),
    ]
    return { from: word.from, options, validFor: /^[\w$]*$/ }
  }
}

// The collection is read out of the command the caret is in rather than out of the whole script, so
// a second command below it does not offer the first one's fields.
function namedCollection(
  before: string,
  byName: Map<string, TableSchema>,
): TableSchema | undefined {
  const command = before.slice(before.lastIndexOf(';') + 1)
  const name = /\bdb\.([\w$]+)\./.exec(command)?.[1]
  return name ? byName.get(name.toLowerCase()) : undefined
}

function methodOptions(methods: ReadonlyArray<[string, string]>): Completion[] {
  return methods.map(([label, detail]) => ({ label, type: 'method', detail }))
}

// No source belongs inside a string or a comment, where nothing being typed is a name at all.
const OFF_LIMITS = ['String', 'TemplateString', 'LineComment', 'BlockComment']

/**
 * The operators, coloured — which is the one thing highlighting a document store cannot get from a
 * grammar. To JavaScript `$gt` is a key like any other, and to the reader it is the whole of what
 * the filter *says*, so it is marked as the keyword it is.
 *
 * **Nothing here reads the script.** The nodes come out of the tree lang-javascript already parsed,
 * which is what keeps `"$total"` a string and `// $set` a comment — and is why this is not a second
 * scanner; see the rule in CLAUDE.md.
 */
const KEYS = new Set(['PropertyDefinition', 'PropertyName'])

const operatorMark = Decoration.mark({ class: 'cm-operator' })

function operators(view: EditorView): DecorationSet {
  const builder = new RangeSetBuilder<Decoration>()

  for (const { from, to } of view.visibleRanges) {
    syntaxTree(view.state).iterate({
      from,
      to,
      enter: (node) => {
        if (KEYS.has(node.name) && view.state.doc.sliceString(node.from, node.from + 1) === '$') {
          builder.add(node.from, node.to, operatorMark)
        }
      },
    })
  }

  return builder.finish()
}

// The marks follow the viewport as well as the text, because only the visible ranges are walked —
// and they follow the tree, which on a long script finishes parsing after the change that began it.
const operatorHighlight = ViewPlugin.fromClass(
  class {
    marks: DecorationSet

    constructor(view: EditorView) {
      this.marks = operators(view)
    }

    update(update: ViewUpdate) {
      const reparsed = syntaxTree(update.startState) !== syntaxTree(update.state)
      if (update.docChanged || update.viewportChanged || reparsed) {
        this.marks = operators(update.view)
      }
    }
  },
  { decorations: (plugin) => plugin.marks },
)

const operatorTheme = EditorView.baseTheme({ '.cm-operator': { color: 'var(--syn-keyword)' } })

export function shellLanguage(
  collections: TableSchema[],
  library: readonly Snippet[],
): Extension {
  const support = javascript()

  return [
    support,
    operatorHighlight,
    operatorTheme,
    support.language.data.of({
      autocomplete: ifNotIn(OFF_LIMITS, afterCollection),
    }),
    support.language.data.of({
      autocomplete: ifNotIn(OFF_LIMITS, afterDb(collections)),
    }),
    support.language.data.of({
      autocomplete: ifNotIn(OFF_LIMITS, afterCall),
    }),
    support.language.data.of({
      autocomplete: ifNotIn(OFF_LIMITS, operatorSource),
    }),
    support.language.data.of({
      autocomplete: ifNotIn(OFF_LIMITS, completeFromList([...SNIPPETS, ...saved(library)])),
    }),
    support.language.data.of({
      autocomplete: ifNotIn(OFF_LIMITS, fieldSource(collections)),
    }),
  ]
}
