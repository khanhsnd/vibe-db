import { javascriptLanguage } from '@codemirror/lang-javascript'
import { jsonLanguage } from '@codemirror/lang-json'
import type { LRLanguage } from '@codemirror/language'

/** Which notation a laid-out value is in, which is the engine's and not the column's. */
export type Notation = 'json' | 'shell'

/**
 * The grammar each notation is read by. Both are the library's — **nothing here scans anything**,
 * which is the rule — and they live in one file because two things read them: `DocumentView` lays a
 * document out with them, and `documentPage` walks the same tree to draw one as a page. One grammar,
 * one place, so a document cannot be laid out under one reading and drawn under another.
 *
 * `shell` is a document store's own notation, and it is read as a **single expression** rather than
 * as a script: read as a script a bare `{ … }` is a *block*, its first field a label and every one
 * after it a syntax error, which is why a document's keys come out uncoloured and half its lines
 * squiggled. `SingleExpression` is the parser's own top rule for exactly this, and under it the
 * document is the object it plainly is.
 */
export const LANGUAGES: Record<Notation, LRLanguage> = {
  json: jsonLanguage,
  shell: javascriptLanguage.configure({ top: 'SingleExpression' }),
}
