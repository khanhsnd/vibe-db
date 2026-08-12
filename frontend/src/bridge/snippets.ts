import * as api from '@wailsjs/go/app/QueryAPI'
import type { app } from '@wailsjs/go/models'

import { call } from './errors'

/**
 * One statement the reader has named and kept. Where a `HistoryEntry` is what happened, a snippet is
 * what somebody meant to come back to — so it carries no connection, no namespace and no outcome: a
 * statement worth naming is one worth running in more than one place.
 *
 * `engine` is `''` for a snippet offered in front of every engine, and otherwise the one it is for.
 * It is the only narrowing there is, and it exists because the library is offered by the editor's own
 * completion: a `SELECT` in front of a Redis tab is noise.
 */
export interface Snippet {
  name: string
  description: string
  /** The text put in the editor, verbatim. `${…}` marks a field the caret jumps between with `Tab`. */
  body: string
  engine: string
}

/**
 * A snippet on its way in. `previous` is the name it was known by — `''` for one that is new — so a
 * rename replaces the old entry in the same write rather than leaving two behind.
 */
export interface SnippetInput {
  previous: string
  name: string
  description: string
  body: string
  engine: string
}

/** The one place a snippet crosses into the generated bindings — see the note on `ProfileInput`. */
const wire = (input: SnippetInput) => input as unknown as app.SnippetInput

/**
 * **Every write answers with the library it leaves behind.** The list is held in one store shared by
 * every query tab and by the editor's completion, so a save that answered with nothing would leave
 * that store to read again — and the reader's completion offering yesterday's names until it did.
 */
export const snippets = {
  list: (): Promise<Snippet[]> =>
    call(() => api.ListSnippets()) as unknown as Promise<Snippet[]>,
  save: (input: SnippetInput): Promise<Snippet[]> =>
    call(() => api.SaveSnippet(wire(input))) as unknown as Promise<Snippet[]>,
  remove: (name: string): Promise<Snippet[]> =>
    call(() => api.DeleteSnippet(name)) as unknown as Promise<Snippet[]>,
}

/** A snippet with nothing filled in, which is what the New dialog opens on. */
export function emptySnippet(engine = ''): SnippetInput {
  return { previous: '', name: '', description: '', body: '', engine }
}

export function toSnippetInput(one: Snippet): SnippetInput {
  return { previous: one.name, ...one }
}
