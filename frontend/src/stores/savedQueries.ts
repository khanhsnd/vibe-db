import { NO_NAMESPACE, type Namespace } from '@/bridge/connections'

import type { QueryTab } from './tabs'

/**
 * The query tabs the app opens again next time it starts.
 *
 * A query tab **is** its buffer: nothing else in the app holds what was typed there, and a reader who
 * closes the window mid-script has not asked for it to be thrown away. So the strip's query tabs are
 * written down as they are edited and read back as the store is built — the tab in front included,
 * because being sent back to the first tab is being sent somewhere the reader was not.
 *
 * Only the **question** survives, never the answer: the rows a statement returned are
 * `stores/queries.ts`'s and are not stored, for the reason a table tab re-reads on the way back — a
 * result kept across a restart would be an account of a server as it was, drawn as if it were now.
 *
 * Only *query* tabs are kept. Every other kind names something on a server — a table, a definition, a
 * comparison — and holds no text of its own, so it is opened again from the tree in one press rather
 * than restored against a connection that may no longer be dialled.
 */

const STORE_KEY = 'vibe-db.queryTabs'

/** How long a burst of typing is allowed to run before it is written down. */
const SAVE_DELAY_MS = 400

/** A query tab as it is written down: everything the tab *is*, less the id it is given when it opens. */
export type SavedQuery = Omit<QueryTab, 'id' | 'kind'>

export interface SavedQueries {
  tabs: SavedQuery[]
  /** Which of them was in front, as a place in the list. `-1` where the tab in front was another kind. */
  active: number
}

const NOTHING_SAVED: SavedQueries = { tabs: [], active: -1 }

export function loadQueryTabs(): SavedQueries {
  const raw = localStorage.getItem(STORE_KEY)
  if (raw === null) return NOTHING_SAVED

  try {
    const parsed = JSON.parse(raw) as Partial<SavedQueries>
    if (!Array.isArray(parsed.tabs)) return NOTHING_SAVED

    return {
      tabs: parsed.tabs.flatMap(readQuery),
      active: typeof parsed.active === 'number' ? parsed.active : -1,
    }
  } catch {
    return NOTHING_SAVED
  }
}

// Read a field at a time rather than trusted whole: what is in the entry was written by a version of
// this app, and one of them wrote something else there. A tab that does not read as one is dropped,
// which is what keeps a bad entry from being a window that opens on nothing.
function readQuery(held: unknown): SavedQuery[] {
  if (typeof held !== 'object' || held === null) return []
  const one = held as Record<string, unknown>

  if (typeof one.title !== 'string' || typeof one.profileId !== 'string') return []
  if (typeof one.sql !== 'string' || typeof one.rowLimit !== 'number') return []

  return [
    {
      title: one.title,
      ...(typeof one.pinned === 'boolean' ? { pinned: one.pinned } : {}),
      profileId: one.profileId,
      namespace: readNamespace(one.namespace),
      sql: one.sql,
      rowLimit: one.rowLimit,
    },
  ]
}

function readNamespace(held: unknown): Namespace {
  if (typeof held !== 'object' || held === null) return NO_NAMESPACE
  const { database, schema } = held as Record<string, unknown>

  return typeof database === 'string' && typeof schema === 'string'
    ? { database, schema }
    : NO_NAMESPACE
}

let pending: ReturnType<typeof setTimeout> | undefined
let latest = NOTHING_SAVED

/**
 * Writes the strip down, on its own clock rather than on the keystroke's: every character typed
 * changes a buffer, and a script can be a megabyte of them. What lands is always the **latest**
 * state, so a burst of typing costs one write rather than one per key.
 */
export function saveQueryTabs(saved: SavedQueries): void {
  latest = saved
  if (pending === undefined) pending = setTimeout(flush, SAVE_DELAY_MS)
}

function flush(): void {
  pending = undefined
  localStorage.setItem(STORE_KEY, JSON.stringify(latest))
}

// The window closing is the one moment that clock cannot be waited out.
window.addEventListener('beforeunload', () => {
  if (pending === undefined) return
  clearTimeout(pending)
  flush()
})
