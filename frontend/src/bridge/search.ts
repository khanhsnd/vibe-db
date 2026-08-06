import * as api from '@wailsjs/go/app/ConnectionAPI'
import type { app } from '@wailsjs/go/models'

import { call } from './errors'

/**
 * Which half of an object a search reads. They are two different kinds of question — the rows a
 * server holds against the names this app has already read — which is why they are answered in two
 * different places and why the reader picks between them rather than getting both.
 */
export type SearchScope = 'data' | 'structure'

/**
 * How the text a reader typed is matched. A token from the Go side's own closed list, never the
 * pattern itself: nothing typed here reaches a server as SQL.
 */
export type SearchMode = 'contains' | 'word' | 'prefix' | 'regex'

/** What the `Look In` list offers, in its own order. */
export const SEARCH_SCOPES: ReadonlyArray<{ value: SearchScope; label: string }> = [
  { value: 'data', label: 'Data' },
  { value: 'structure', label: 'Structure' },
]

/** What the `Search Mode` list offers, in its own order. */
export const SEARCH_MODES: ReadonlyArray<{ value: SearchMode; label: string }> = [
  { value: 'contains', label: 'Contains' },
  { value: 'word', label: 'Whole Word' },
  { value: 'prefix', label: 'Prefix' },
  { value: 'regex', label: 'Regular Expression' },
]

/** Whether this mode reads `%` and `_` as wildcards, which is what the escape note is about. */
export function isPatternMode(mode: SearchMode): boolean {
  return mode === 'contains' || mode === 'prefix'
}

/**
 * One Find in Schema. `runId` is minted per search so it can be stopped before it answers: the call
 * only returns once every table of the namespace has been read.
 */
export interface SearchRequest {
  runId: string
  database: string
  schema: string
  text: string
  scope: SearchScope
  mode: SearchMode
  caseInsensitive: boolean
}

/**
 * One object the search matched, and how many of its rows or declared names did.
 *
 * `statement` is the read that returns exactly the rows counted, spelled by the **engine** from the
 * same condition the count was made through — which is what a double-click opens in a query tab, and
 * why the tab cannot show a different set of rows from the number that led the reader to it. A
 * structure hit carries none: matching names is not a read.
 */
export interface SearchHit {
  table: string
  matches: number
  statement: string
}

/**
 * One object the search could not read, carrying the server's own wording. A schema nearly always
 * holds something the account may not read, and stopping at the first one would make the feature
 * unusable there — so it is reported and the search carries on.
 */
export interface SearchSkip {
  table: string
  error: string
}

export interface SearchResult {
  hits: SearchHit[]
  /** How many objects were read, so the line under the results can say one *of* forty. */
  searched: number
  skipped: SearchSkip[]
}

export const search = {
  find: (sessionId: string, request: SearchRequest): Promise<SearchResult> =>
    call(() =>
      api.FindInSchema(sessionId, request as unknown as app.SearchInput),
    ) as unknown as Promise<SearchResult>,

  /** Stops a search before it answers. An id the backend has forgotten is ignored. */
  stop: (runId: string): Promise<void> => call(() => api.StopSearch(runId)),
}
