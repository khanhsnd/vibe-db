import type { Extension } from '@codemirror/state'
import { placeholder } from '@codemirror/view'

import { engineInfo, type TableSchema } from '@/bridge/connections'

import { shellLanguage } from './shellLanguage'
import { sqlLanguage } from './sqlLanguage'

/**
 * The language the editor is in, which is the engine's. Two of the three write SQL and differ only
 * in a dialect and a quoting rule; the third writes commands over documents and shares none of that,
 * so it gets its own highlighting and its own completion.
 *
 * This is the whole of what the editor knows about which engine it is in front of — the empty tab's
 * own prompt included, since `SELECT * FROM` is not what a document store is waiting for. **Nothing
 * here decides what a command *does*** — that is the driver's `Review`, which the editor asks for
 * and never re-derives; see docs/editor.md.
 */
export function queryLanguage(tables: TableSchema[], engine: string): Extension {
  if (engineInfo(engine).documents) {
    return [placeholder('db.collection.find({ ... })'), shellLanguage(tables)]
  }
  return [placeholder('SELECT * FROM ...'), sqlLanguage(tables, engine)]
}
