/**
 * A document drawn as a **page** rather than read as text: every field on a line of its own, and
 * whatever a field holds shown as the thing it is — markup as the markup it is, an image as the
 * image it is, and a value nothing can draw as an empty square saying so.
 *
 * It exists because a document with markup inside it reads badly *either* way round. Laid out, a
 * field holding a page is one enormous escaped line among the fields either side of it; drawn as a
 * page, a document is only the one field and the rest of it is gone. So the page is the whole
 * document, and the markup inside it is drawn where the field that holds it is.
 *
 * **Nothing here scans anything.** The structure comes from the same grammar `DocumentView` lays a
 * document out with — `LANGUAGES`, the library's — walked as a tree, so a bare key, an `ObjectId(…)`
 * and where a string ends are the parser's answers and not this file's. A value that is not an
 * object or an array is drawn as **the engine's own words**, verbatim off the text.
 */

import { bytesLabel, dataURI, decodeBinary, holdsMarkup, isBinary } from './value'
import { LANGUAGES, type Notation } from './notation'

/** The one node type this needs, taken off the parser so nothing imports the tree package. */
type Node = ReturnType<(typeof LANGUAGES)['json']['parser']['parse']>['topNode']

/** What an object is called in each grammar. The two differ in this and in nothing else that matters. */
const OBJECT = new Set(['Object', 'ObjectExpression'])
const ARRAY = new Set(['Array', 'ArrayExpression'])

/**
 * The page's own ground: a field's name in the colour a document's keys are read in, a rule for each
 * shape a value can take, and a box for the ones that have none. Every colour is a token the frame
 * was given — see `HtmlView` — because a colour written here would be a colour outside
 * `tokens.css`.
 *
 * It is a `<style>` of the page rather than a rule of the frame so that `HtmlView` stays what it is:
 * one policy, one set of tokens, and the markup it was handed.
 */
const STYLE = `<style>
.f{display:flex;gap:10px;padding:2px 0;align-items:baseline}
.k{flex:0 0 auto;min-width:7rem;font-family:var(--font-data);color:var(--syn-func)}
.v{flex:1;min-width:0}
.n{display:block}
.n>.v{margin:2px 0 6px 10px;border-left:1px solid var(--line);padding-left:10px}
.t{white-space:pre-wrap;word-break:break-word}
.s{font-family:var(--font-data);color:var(--fg-muted)}
.p{border:1px solid var(--line);border-radius:4px;padding:10px;overflow-x:auto}
.b{display:inline-flex;align-items:center;gap:6px;min-width:1.25rem;min-height:1.25rem;padding:0 6px;
border:1px solid var(--line-strong);border-radius:3px;color:var(--fg-faint);font-size:11px}
img{max-width:100%}
</style>`

/** The document as the markup of a page. `notation` is the engine's, exactly as `DocumentView`'s is. */
export function documentMarkup(text: string, notation: Notation): string {
  const tree = LANGUAGES[notation].parser.parse(text)
  return STYLE + drawn(contents(tree.topNode), text)
}

/**
 * The value itself, under the wrapper each grammar's top rule puts around it — `JsonText` for one and
 * `SingleExpression` for the other. A document that would not parse has no first child and is drawn
 * as the words it is, which is the honest answer and never a blank page.
 */
function contents(top: Node): Node {
  const first = top.firstChild
  return first !== null && (OBJECT.has(first.name) || ARRAY.has(first.name)) ? first : top
}

function drawn(node: Node, text: string): string {
  if (OBJECT.has(node.name)) return group(fields(node, text))
  if (ARRAY.has(node.name)) return group(items(node, text))
  return leaf(text.slice(node.from, node.to))
}

/** Whether a value is a group of its own, which is what puts it under its name instead of beside it. */
function nested(node: Node | undefined): boolean {
  return node !== undefined && (OBJECT.has(node.name) || ARRAY.has(node.name))
}

function fields(node: Node, text: string): string[] {
  return children(node).map((property) => {
    const [name, held] = children(property)
    if (name === undefined) return ''

    const label = unquote(text.slice(name.from, name.to)) ?? text.slice(name.from, name.to)
    return row(label, held === undefined ? box('□') : drawn(held, text), nested(held))
  })
}

/** An array's rows are numbered, exactly as the `Items` view numbers a PostgreSQL array's. */
function items(node: Node, text: string): string[] {
  return children(node).map((item, index) => row(String(index), drawn(item, text), nested(item)))
}

function group(rows: string[]): string {
  if (rows.length === 0) return box('□')
  return rows.join('')
}

/**
 * One field: its name, and what it holds. A value that is itself a group goes **under** the name and
 * indented rather than beside it, because a column of names beside a column of names beside a column
 * of names is what a document three deep would otherwise be, and by the third one there is no room
 * left for the values.
 */
function row(name: string, value: string, nest: boolean): string {
  const shape = nest ? 'f n' : 'f'
  return `<div class="${shape}"><div class="k">${escaped(name)}</div><div class="v">${value}</div></div>`
}

/**
 * One value that is neither an object nor an array. A string is drawn as what it holds — markup as
 * markup, bytes as the image they are — and everything else is drawn as the engine wrote it:
 * a number, `true`, `null`, an `ObjectId(…)`, an `ISODate(…)`.
 *
 * **A field is drawn as markup wherever it holds a tag**, not only where it opens with one. Whether a
 * *cell* is a page is a different question and takes the stricter rule — see `value.ts` — because
 * there the whole value has to be markup or nothing is; here the reader asked for the page.
 */
function leaf(source: string): string {
  const held = unquote(source)
  if (held === null) return `<span class="s">${escaped(source)}</span>`
  if (held === '') return box('□')

  if (holdsMarkup(held)) return `<div class="p">${held}</div>`

  if (isBinary(held)) {
    const bytes = decodeBinary(held)
    const image = dataURI(bytes)
    return image === null
      ? box(`□ ${bytesLabel(bytes.length)}`)
      : `<img src="${image}" alt="${escaped(bytesLabel(bytes.length))}">`
  }

  return `<span class="t">${escaped(held)}</span>`
}

/** What is shown where there is nothing to draw: a square, and a word about why where there is one. */
function box(label: string): string {
  return `<span class="b">${escaped(label)}</span>`
}

/**
 * What a string literal holds, or `null` where the source is not one. The unescaping is the
 * platform's: the driver writes double quotes throughout so that what it writes is JSON as well as
 * shell — `mongo.quote` — and a literal a reader typed in some other way is taken as the text
 * between its quotes.
 */
function unquote(source: string): string | null {
  const quote = source[0]
  if (quote !== '"' && quote !== "'") return null

  try {
    return JSON.parse(source) as string
  } catch {
    return source.slice(1, source.endsWith(quote) ? -1 : undefined)
  }
}

/** The word-named children, which is every child that is a value and none that is punctuation. */
function children(node: Node): Node[] {
  const found: Node[] = []
  for (let child = node.firstChild; child !== null; child = child.nextSibling) {
    if (/^[A-Za-z]/.test(child.name)) found.push(child)
  }
  return found
}

function escaped(text: string): string {
  return text.replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
}
