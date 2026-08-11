import { create } from 'zustand'

import type { TablePage } from '@/bridge/tables'

/** The read a held page answers to when it answers nothing — what Refresh leaves behind. */
const STALE = ''

/** The page a table tab has in hand: what came back, what asked for it, and when it landed. */
export interface HeldPage {
  /** The read this answers, as `useTableData` spells one. `STALE` is a page that answers none. */
  request: string
  data: TablePage
  loadedAt: number
}

interface TableRowsState {
  pages: Record<string, HeldPage>
  hold: (tabId: string, page: HeldPage) => void
  /** Leaves the rows on screen but makes them answer nothing, which is what Refresh means. */
  stale: (tabId: string) => void
  discard: (tabId: string) => void
}

/**
 * The rows a table tab has in hand, keyed by the tab id.
 *
 * It is a store rather than the view's own state because **only the active tab is mounted**: a page
 * kept in a component goes down with it, and the read that replaced it on the way back took the
 * reader's un-applied rows with it. A tab therefore keeps its answer beside what was asked for it —
 * see `stores/tableViews.ts` — and reads again only when the request changes or Refresh says so.
 */
export const useTableRowsStore = create<TableRowsState>((set) => ({
  pages: {},

  hold: (tabId, page) => set((s) => ({ pages: { ...s.pages, [tabId]: page } })),

  stale: (tabId) =>
    set((s) => {
      const held = s.pages[tabId]
      return held === undefined
        ? s
        : { pages: { ...s.pages, [tabId]: { ...held, request: STALE } } }
    }),

  discard: (tabId) =>
    set((s) => {
      const { [tabId]: gone, ...rest } = s.pages
      return gone === undefined ? s : { pages: rest }
    }),
}))
