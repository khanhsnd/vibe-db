import type { Extension } from '@codemirror/state'
import { placeholder } from '@codemirror/view'

import { engineInfo } from '@/bridge/connections'
import type { Snippet } from '@/bridge/snippets'

import { commandLanguage } from './commandLanguage'
import { shellLanguage } from './shellLanguage'
import { sqlLanguage, type QuerySchema } from './sqlLanguage'

/**
 * The language the editor is in, which is the engine's. Two of the four write SQL and differ only
 * in a dialect and a quoting rule; the third writes commands over documents and the fourth writes
 * argv, and neither shares any of that — so each gets its own highlighting and its own completion.
 *
 * This is the whole of what the editor knows about which engine it is in front of — the empty tab's
 * own prompt included, since `SELECT * FROM` is not what a document store is waiting for. **Nothing
 * here decides what a command *does*** — that is the driver's `Review`, which the editor asks for
 * and never re-derives; see docs/editor.md.
 *
 * `library` is the snippets the reader has named, already narrowed to the ones this engine may be
 * offered — see `snippetsFor`. All three languages take it, because a name somebody chose is offered
 * the same way whatever it holds; only what surrounds it in the list differs.
 */
export function queryLanguage(
  schema: QuerySchema,
  engine: string,
  library: readonly Snippet[],
): Extension {
  const info = engineInfo(engine)

  if (info.keys) {
    return [placeholder('SCAN 0 MATCH * COUNT 100'), commandLanguage(library)]
  }
  if (info.documents) {
    return [placeholder('db.collection.find({ ... })'), shellLanguage(schema.tables, library)]
  }
  return [placeholder('SELECT * FROM ...'), sqlLanguage(schema, engine, library)]
}
