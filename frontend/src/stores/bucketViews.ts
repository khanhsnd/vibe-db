import { create } from 'zustand'

import { toAppError, type AppError } from '@/bridge/errors'
import {
  DEFAULT_SEARCH_MODE,
  objects,
  type ObjectPage,
  type ObjectSearchResult,
} from '@/bridge/objects'

/**
 * What a bucket tab was set up with and what came back, kept per tab id — the arrangement
 * `stores/tableViews.ts` and `stores/tableRows.ts` have, and for the same reason: `WorkArea` mounts
 * the tab in front and no other, so a component's own state goes down with it.
 *
 * The rows live here beside the view rather than in a store of their own, which is where this differs
 * from a table tab. A listing is still **forward only** — that is what a continuation token is — but
 * the reader never sees a page: `more` appends the next one as the grid is scrolled, so what is held
 * is every page read so far, merged. There is still no `All`, because a bucket has no size anybody
 * can ask for — what stops the reading is the reader stopping scrolling.
 */
export interface BucketSearch {
  text: string
  mode: string
  caseInsensitive: boolean
  /** Bytes, and zero for no bound. */
  minSize: number
  maxSize: number
  /** RFC 3339, or empty for no bound. */
  since: string
  until: string
}

export const NO_SEARCH: BucketSearch = {
  text: '',
  mode: DEFAULT_SEARCH_MODE,
  caseInsensitive: false,
  minSize: 0,
  maxSize: 0,
  since: '',
  until: '',
}

export interface BucketView {
  bucket: string
  /** The folder the reader is standing in, ending in `/`, and `''` at the top of the bucket. */
  prefix: string
  /** Everything under the prefix rather than the folder alone. */
  flat: boolean
  /**
   * Every page read so far as one, and `null` before the first. Its `next` is what the page after it
   * is asked for with, and empty once the folder has been read to the end — see `merge`.
   */
  page: ObjectPage | null
  loading: boolean

  search: BucketSearch
  /** Non-null exactly while a search's answer is what the grid is drawing. */
  result: ObjectSearchResult | null
  searching: boolean
  /** Minted per search so it can be stopped — the arrangement a read of every row has. */
  runId: string

  /** The keys the reader has ticked, which is what every action is addressed to. */
  picked: string[]
  /** The row the panel under the grid is open on. */
  focused: string | null
  error: AppError | null
}

function emptyView(bucket: string, prefix: string): BucketView {
  return {
    bucket,
    prefix,
    flat: false,
    page: null,
    loading: false,
    search: NO_SEARCH,
    result: null,
    searching: false,
    runId: '',
    picked: [],
    focused: null,
    error: null,
  }
}

interface BucketViewsState {
  views: Record<string, BucketView>

  /** Starts a tab's view, or leaves the one it already has alone. */
  open: (tabId: string, bucket: string, prefix: string) => void
  drop: (tabId: string) => void

  read: (tabId: string, sessionId: string) => Promise<void>
  goTo: (tabId: string, sessionId: string, prefix: string) => Promise<void>
  up: (tabId: string, sessionId: string) => Promise<void>
  more: (tabId: string, sessionId: string) => Promise<void>
  setFlat: (tabId: string, sessionId: string, flat: boolean) => Promise<void>

  setSearch: (tabId: string, search: Partial<BucketSearch>) => void
  runSearch: (tabId: string, sessionId: string) => Promise<void>
  stopSearch: (tabId: string) => Promise<void>
  clearSearch: (tabId: string, sessionId: string) => Promise<void>

  pick: (tabId: string, keys: string[]) => void
  focus: (tabId: string, key: string | null) => void
}

export const useBucketViewsStore = create<BucketViewsState>((set, get) => ({
  views: {},

  open: (tabId, bucket, prefix) => {
    if (get().views[tabId]) return
    set((s) => ({ views: { ...s.views, [tabId]: emptyView(bucket, prefix) } }))
  },

  drop: (tabId) =>
    set((s) => {
      const views = { ...s.views }
      delete views[tabId]
      return { views }
    }),

  /** The folder from its first page, replacing whatever had been read of it before. */
  read: async (tabId, sessionId) => {
    const view = get().views[tabId]
    if (!view) return

    change(set, tabId, { loading: true, error: null })
    try {
      const page = await objects.listObjects(sessionId, request(view, ''))
      change(set, tabId, { page, loading: false, result: null })
    } catch (thrown) {
      change(set, tabId, { loading: false, error: toAppError(thrown) })
    }
  },

  // Walking into a folder reads it from its first page and lets go of what was ticked: a key the
  // reader picked in one folder is not a key in this one.
  goTo: async (tabId, sessionId, prefix) => {
    change(set, tabId, { prefix, picked: [], focused: null, result: null })
    await get().read(tabId, sessionId)
  },

  up: async (tabId, sessionId) => {
    const view = get().views[tabId]
    if (!view || view.prefix === '') return

    const trimmed = view.prefix.replace(/\/$/, '')
    const at = trimmed.lastIndexOf('/')
    await get().goTo(tabId, sessionId, at < 0 ? '' : trimmed.slice(0, at + 1))
  },

  /**
   * The page after the one on screen, **appended**. It is what the grid asks for as it is scrolled
   * near the bottom, so a reader walking a folder of forty thousand objects presses nothing — and it
   * is the whole of why there is no pager: a page number needs a count of the pages, and a bucket
   * has none to give.
   *
   * It reads the tab **again** after the await and drops the answer if the reader has walked
   * somewhere else meanwhile. A page of the folder they left appended to the folder they are in
   * would be two places in one grid, and nothing afterwards could tell which row was which.
   */
  more: async (tabId, sessionId) => {
    const view = get().views[tabId]
    if (!view?.page?.next || view.loading) return

    change(set, tabId, { loading: true, error: null })
    try {
      const page = await objects.listObjects(sessionId, request(view, view.page.next))
      const held = get().views[tabId]
      if (!held || held.prefix !== view.prefix || held.flat !== view.flat) return

      change(set, tabId, { page: merge(held.page, page), loading: false })
    } catch (thrown) {
      change(set, tabId, { loading: false, error: toAppError(thrown) })
    }
  },

  setFlat: async (tabId, sessionId, flat) => {
    change(set, tabId, { flat, picked: [], result: null })
    await get().read(tabId, sessionId)
  },

  setSearch: (tabId, search) =>
    set((s) => {
      const view = s.views[tabId]
      if (!view) return s
      return { views: { ...s.views, [tabId]: { ...view, search: { ...view.search, ...search } } } }
    }),

  /**
   * A search reads the bucket rather than the folder, so it is **started** rather than typed into:
   * every letter narrowing a read of a million keys would be a letter that cost a minute. The box
   * commits on Enter and on the button, which is what the panel it replaces does with Apply.
   */
  runSearch: async (tabId, sessionId) => {
    const view = get().views[tabId]
    if (!view || view.search.text.trim() === '') return

    const runId = `search-${tabId}-${Date.now()}`
    change(set, tabId, { searching: true, runId, error: null, picked: [], focused: null })

    try {
      const result = await objects.search(sessionId, {
        bucket: view.bucket,
        prefix: view.prefix,
        text: view.search.text,
        mode: view.search.mode,
        caseInsensitive: view.search.caseInsensitive,
        minSize: view.search.minSize,
        maxSize: view.search.maxSize,
        since: view.search.since,
        until: view.search.until,
        hits: 0,
        scan: 0,
        runId,
      })
      change(set, tabId, { result, searching: false })
    } catch (thrown) {
      change(set, tabId, { searching: false, error: toAppError(thrown) })
    }
  },

  stopSearch: async (tabId) => {
    const view = get().views[tabId]
    if (!view?.runId) return
    await objects.stopRun(view.runId)
  },

  clearSearch: async (tabId, sessionId) => {
    change(set, tabId, { search: NO_SEARCH, result: null, picked: [], focused: null })
    await get().read(tabId, sessionId)
  },

  pick: (tabId, keys) => change(set, tabId, { picked: keys }),
  focus: (tabId, key) => change(set, tabId, { focused: key }),
}))

/** One tab's view, changed in place. Every action here goes through it, so nothing else spreads. */
function change(
  set: (fn: (state: BucketViewsState) => Partial<BucketViewsState>) => void,
  tabId: string,
  edit: Partial<BucketView>,
) {
  set((s) => {
    const view = s.views[tabId]
    if (!view) return s
    return { views: { ...s.views, [tabId]: { ...view, ...edit } } }
  })
}

/** One folder asked for from a given point, which `read` and `more` differ only in. */
function request(view: BucketView, token: string) {
  return { bucket: view.bucket, prefix: view.prefix, token, limit: 0, flat: view.flat }
}

/**
 * Two pages of one folder as one. The rows are **partitioned and not concatenated**: a service
 * answers a listing in its own single order, so the folders of the second page would otherwise land
 * under the objects of the first — and folders come first here for the reason every file manager
 * puts them there. Each half keeps the order the service answered in, `filter` being stable.
 *
 * The three totals add up, so the footer counts everything read rather than the last page alone.
 * `next` is the newer page's, that being the only one that continues anything.
 */
function merge(held: ObjectPage | null, page: ObjectPage): ObjectPage {
  if (held === null) return page

  const rows = [...held.rows, ...page.rows]
  return {
    ...page,
    rows: [...rows.filter((row) => row.folder), ...rows.filter((row) => !row.folder)],
    folders: held.folders + page.folders,
    objects: held.objects + page.objects,
    bytes: held.bytes + page.bytes,
  }
}

/** The rows the grid is drawing: a search's answer where there is one, and the page otherwise. */
export function rowsOf(view: BucketView) {
  return view.result?.rows ?? view.page?.rows ?? []
}
