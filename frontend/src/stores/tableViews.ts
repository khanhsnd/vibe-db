import { create } from 'zustand'

import { DEFAULT_PAGE_SIZE } from '@/bridge/rows'
import { NO_FILTER, type Group } from '@/features/results/filter'
import { NO_SORT, type SortKey } from '@/features/results/resultSet'
import { DEFAULT_KEY_MATCH, type KeyMatch } from '@/features/table/keySpace'
import type { PanelKind } from '@/features/table/TableToolbar'

import { useSettingsStore } from './settings'

/**
 * How a table tab has been set up, for as long as that tab is open.
 *
 * It is here rather than inside `TableDataView` because **only the active tab is mounted**: a tab
 * switched away from is taken down, and anything it was holding in component state goes with it —
 * which is the same reason a query tab's rows live in `stores/queries.ts`. A reader who opened the
 * cell editor, hid two columns and narrowed the table would otherwise find all three undone by a
 * glance at another tab.
 *
 * **What a tab keeps is what was asked for, never the answer.** The panels, the hidden columns, the
 * page and its size, the order and the conditions are the reader's own and are kept; the rows are
 * read again on the way back, so a table something else has changed under it reads fresh rather
 * than stale. A query tab is the other way round — a statement cannot be re-run behind the reader's
 * back — and that is the whole difference between the two stores.
 */
export interface TableView {
  /** The side panels showing: the cell editor, the column list. */
  panels: ReadonlySet<PanelKind>
  filterShown: boolean
  /** Columns the reader took off the grid, by name — a name outlives the column list being re-read. */
  hidden: ReadonlySet<string>
  page: number
  limit: number
  sort: readonly SortKey[]
  criteria: Group
  /**
   * Whether a **key space** is drawn as plain rows rather than as the tree its keys group into. It is
   * the tree that is the default there, a Redis key being a path spelled with colons, and this is the
   * reader asking to see the page as it actually is. It means nothing on any other engine, where the
   * rows have no hierarchy to flatten.
   */
  flatten: boolean
  /** The branches of that tree the reader has folded shut, by their prefix — see `keyTree.ts`. */
  shut: ReadonlySet<string>
  /**
   * How the **key space** search bar reads what is typed in it — see `keySpace.ts`. It is here
   * rather than in the condition it writes because an empty box writes no condition at all, and a
   * reader who picks a mode before typing a word would otherwise lose it to the next keystroke.
   */
  keyMatch: KeyMatch
}

const NO_PANELS: ReadonlySet<PanelKind> = new Set()
const NOTHING_HIDDEN: ReadonlySet<string> = new Set()
const NOTHING_SHUT: ReadonlySet<string> = new Set()

/**
 * A tab nobody has set up yet, and a stable snapshot for one whose view has not been started —
 * a selector that built this per call would render for ever. It carries the shipped page size
 * rather than none, so a view that somehow went unstarted still reads a page rather than a table.
 */
export const IDLE_VIEW: TableView = {
  panels: NO_PANELS,
  filterShown: false,
  hidden: NOTHING_HIDDEN,
  page: 0,
  limit: DEFAULT_PAGE_SIZE,
  sort: NO_SORT,
  criteria: NO_FILTER,
  flatten: false,
  shut: NOTHING_SHUT,
  keyMatch: DEFAULT_KEY_MATCH,
}

interface TableViewsState {
  views: Record<string, TableView>
  /** Gives a newly opened tab its view. A tab being reused keeps the one it already has. */
  start: (tabId: string) => void
  change: (tabId: string, edit: Partial<TableView>) => void
  discard: (tabId: string) => void
}

export const useTableViewsStore = create<TableViewsState>((set) => ({
  views: {},

  start: (tabId) =>
    set((s) =>
      s.views[tabId]
        ? s
        : {
            views: {
              ...s.views,
              [tabId]: { ...IDLE_VIEW, limit: useSettingsStore.getState().settings.pageSize },
            },
          },
    ),

  change: (tabId, edit) =>
    set((s) => ({ views: { ...s.views, [tabId]: { ...(s.views[tabId] ?? IDLE_VIEW), ...edit } } })),

  discard: (tabId) =>
    set((s) => {
      const { [tabId]: gone, ...rest } = s.views
      return gone === undefined ? s : { views: rest }
    }),
}))
