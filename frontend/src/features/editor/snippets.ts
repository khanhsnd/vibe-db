import { snippetCompletion, type Completion } from '@codemirror/autocomplete'

import type { Snippet } from '@/bridge/snippets'

/**
 * A snippet as the editor offers one. There are two kinds and they are offered the same way, which is
 * the whole point of this file: the dozen this app ships with — `select`, `insert`, `join` — and the
 * ones the reader named themselves, which are [the library](../../../docs/features/snippets.md).
 *
 * `${…}` in a body marks a field the caret jumps between with `Tab`. It is the notation the built-in
 * snippets have always used, and the reader's own bodies are read the same way, so a snippet saved out
 * of a working statement can be turned into a form by wrapping the parts that change.
 */

/** One of the snippets this app ships with. The label is what is typed to reach it, so it is lower case. */
export function builtIn(label: string, body: string): Completion {
  return snippetCompletion(body, { label, type: 'snippet', detail: firstLine(body) })
}

/**
 * The reader's own snippets, as completions. They are **boosted above** everything else offered at the
 * same word: a name somebody chose is a name they meant, and having to scroll past `SELECT` to reach
 * it would be the library not being worth using.
 *
 * The description stands as the detail line where there is one, since that is what the reader wrote to
 * tell the two apart; the first line of the body stands in where there is not.
 */
export function saved(library: readonly Snippet[]): Completion[] {
  return library.map((one) =>
    snippetCompletion(one.body, {
      label: one.name,
      type: 'snippet',
      detail: one.description === '' ? firstLine(one.body) : one.description,
      boost: 2,
    }),
  )
}

/**
 * What the completion list shows beside the name: the body's first line, with the placeholder braces
 * taken off so it reads as a statement rather than as a template.
 */
function firstLine(body: string): string {
  const line = body.split('\n')[0] ?? ''
  return line.replace(/\$\{(\w*)\}/g, '$1')
}
