import { create } from 'zustand'

import { namespaceLabel, type Namespace } from '@/bridge/connections'

import { useConsoleStore } from './console'
import { useQueriesStore } from './queries'
import { useTableViewsStore } from './tableViews'

export type TabKind = 'query' | 'table' | 'design' | 'objects' | 'console' | 'compare'

/** Which half of two tables a comparison is reading: their shapes, or their rows. */
export type CompareMode = 'structure' | 'data'

interface TabBase {
  id: string
  title: string
  profileId: string
  namespace: Namespace
}

export interface QueryTab extends TabBase {
  kind: 'query'
  sql: string
  rowLimit: number
}

export interface TableTab extends TabBase {
  kind: 'table'
  table: string
}

/**
 * A table's shape rather than its rows — what Design Table opens. An empty `table` is a table
 * being designed that does not exist yet, which is what New Table opens.
 */
export interface DesignTab extends TabBase {
  kind: 'design'
  table: string
}

/**
 * Every table of a namespace with what the catalogue keeps about each — what the Tables folder
 * opens. It names no table of its own: the namespace is the whole of what it lists.
 */
export interface ObjectsTab extends TabBase {
  kind: 'objects'
}

/**
 * One statement at a time against a connection, with everything already sent kept above the
 * prompt — what Console opens. It holds no text of its own: what was typed has either been sent,
 * and is in the transcript, or is still in the prompt.
 */
export interface ConsoleTab extends TabBase {
  kind: 'console'
}

/**
 * Two tables read side by side — what Compare opens. The tab's own connection, namespace and
 * table are the **left** side, which is the side the reader reads down; `right` is what it is
 * being compared against and carries a connection of its own, because comparing a staging table
 * against the production one is the whole point and those are two servers.
 *
 * `mode` is state rather than identity: one pair is one tab, and asking for the other half of it
 * from the menu reveals that tab and switches it rather than opening a second.
 */
export interface CompareTab extends TabBase {
  kind: 'compare'
  table: string
  right: TableTarget
  mode: CompareMode
}

export type Tab = QueryTab | TableTab | DesignTab | ObjectsTab | ConsoleTab | CompareTab
export type NewTab =
  | Omit<QueryTab, 'id'>
  | Omit<TableTab, 'id'>
  | Omit<DesignTab, 'id'>
  | Omit<ObjectsTab, 'id'>
  | Omit<ConsoleTab, 'id'>
  | Omit<CompareTab, 'id'>

/** What names a table wherever one is opened: which connection, which namespace, which table. */
export interface TableTarget {
  profileId: string
  namespace: Namespace
  table: string
}

/** How a table reads on a tab that names two of them: `shop.users`, or `users` on its own. */
export function targetLabel(target: TableTarget): string {
  return target.namespace.database === ''
    ? target.table
    : `${namespaceLabel(target.namespace)}.${target.table}`
}

/** The arrow between a comparison's two sides, wherever one is drawn. */
export const COMPARE_ARROW = '⇄'

type QueryEdit = Partial<Pick<QueryTab, 'sql' | 'rowLimit'>>

interface TabsState {
  tabs: Tab[]
  activeId: string | null
  open: (tab: NewTab) => string
  /** The rows of a table, or of a view, which reads the same way. */
  openTable: (target: TableTarget) => string
  /** A table's shape. An empty `table` designs one that does not exist yet. */
  openDesign: (target: TableTarget) => string
  /** Every table of a namespace, with what the catalogue keeps about each. */
  openTables: (profileId: string, namespace: Namespace) => string
  /** A console on one connection. One per connection and namespace, like every other tab. */
  openConsole: (profileId: string, namespace: Namespace) => string
  /**
   * Two tables side by side. One pair is one tab, so asking for the other half of a comparison
   * already open reveals it and switches its mode rather than opening a second tab on one pair.
   */
  openCompare: (left: TableTarget, right: TableTarget, mode: CompareMode) => string
  /** Switches an open comparison between the two halves — its toolbar's own control. */
  setCompareMode: (id: string, mode: CompareMode) => void
  close: (id: string) => void
  activate: (id: string) => void
  /** Points a tab at another namespace, which is what the picker on its own strip does. */
  retarget: (id: string, namespace: Namespace) => void
  editQuery: (id: string, edit: QueryEdit) => void
  designed: (id: string, table: string) => void
}

let seq = 0
const nextId = () => `tab-${++seq}`

const tableOf = (tab: Tab | NewTab) => ('table' in tab ? tab.table : '')

const sameTable = (one: TableTarget, other: TableTarget) =>
  one.profileId === other.profileId &&
  one.namespace.database === other.namespace.database &&
  one.namespace.schema === other.namespace.schema &&
  one.table === other.table

// One tab per thing it shows: opening a table twice reveals the tab already showing it, and its
// data tab, its designer and the list it sits in are different kinds of the same namespace. A
// comparison is named by *both* of its sides and not by its mode, which is state inside the tab.
const sameTarget = (tab: Tab, target: NewTab) =>
  tab.kind === target.kind &&
  tab.profileId === target.profileId &&
  tab.namespace.database === target.namespace.database &&
  tab.namespace.schema === target.namespace.schema &&
  tableOf(tab) === tableOf(target) &&
  (tab.kind !== 'compare' || sameTable(tab.right, (target as CompareTab).right))

// A query is never reused — two of them are two buffers — and neither is a table being designed
// that has no name yet, because two of those are two tables.
const reusable = (tab: NewTab) =>
  tab.kind !== 'query' && !(tab.kind === 'design' && tab.table === '')

export const useTabsStore = create<TabsState>((set, get) => ({
  tabs: [],
  activeId: null,

  open: (tab) => {
    if (reusable(tab)) {
      const existing = get().tabs.find((current) => sameTarget(current, tab))
      if (existing) {
        set({ activeId: existing.id })
        return existing.id
      }
    }

    const id = nextId()
    set((s) => ({ tabs: [...s.tabs, { ...tab, id }], activeId: id }))

    // A table tab is given its view here rather than by whatever opened it, so a tab cannot exist
    // without one. What the reader does to a table outlives the tab being looked away from — see
    // `stores/tableViews.ts` — and a tab being reused above keeps the view it already has.
    if (tab.kind === 'table') useTableViewsStore.getState().start(id)
    return id
  },

  openTable: ({ profileId, namespace, table }) =>
    get().open({ kind: 'table', title: table, profileId, namespace, table }),

  openDesign: ({ profileId, namespace, table }) =>
    get().open({
      kind: 'design',
      title: table === '' ? 'New Table' : `${table} · Design`,
      profileId,
      namespace,
      table,
    }),

  // The namespace is not in the title: every tab in the strip draws the one it belongs to, and a
  // title that repeated it would say `hirec.public` twice on one tab.
  openTables: (profileId, namespace) =>
    get().open({ kind: 'objects', title: 'Tables', profileId, namespace }),

  openConsole: (profileId, namespace) =>
    get().open({ kind: 'console', title: 'Console', profileId, namespace }),

  openCompare: (left, right, mode) => {
    const id = get().open({
      kind: 'compare',
      title: `${left.table} ${COMPARE_ARROW} ${right.table}`,
      profileId: left.profileId,
      namespace: left.namespace,
      table: left.table,
      right,
      mode,
    })
    // A pair already open is revealed rather than opened again, so the mode the reader just asked
    // for is applied to the tab they were sent to — otherwise Compare ▸ Data Only on a comparison
    // already showing structure would look like it had done nothing.
    get().setCompareMode(id, mode)
    return id
  },

  setCompareMode: (id, mode) =>
    set((s) => ({
      tabs: s.tabs.map((tab) => (tab.id === id && tab.kind === 'compare' ? { ...tab, mode } : tab)),
    })),

  close: (id) => {
    useQueriesStore.getState().discard(id)
    useTableViewsStore.getState().discard(id)
    useConsoleStore.getState().discard(id)

    set((s) => {
      const index = s.tabs.findIndex((t) => t.id === id)
      if (index === -1) return s

      const tabs = s.tabs.filter((t) => t.id !== id)
      if (s.activeId !== id) return { tabs, activeId: s.activeId }

      const neighbour = tabs[index] ?? tabs[index - 1]
      return { tabs, activeId: neighbour?.id ?? null }
    })
  },

  activate: (id) => set({ activeId: id }),

  retarget: (id, namespace) =>
    set((s) => ({ tabs: s.tabs.map((tab) => (tab.id === id ? { ...tab, namespace } : tab)) })),

  // A design tab whose Save has just created its table is no longer designing a new one: it now
  // designs that table, and pressing Save again alters it rather than creating it twice.
  designed: (id, table) =>
    set((s) => ({
      tabs: s.tabs.map((tab) =>
        tab.id === id && tab.kind === 'design'
          ? { ...tab, table, title: `${table} · Design` }
          : tab,
      ),
    })),

  editQuery: (id, edit) =>
    set((s) => ({
      tabs: s.tabs.map((tab) => (tab.id === id && tab.kind === 'query' ? { ...tab, ...edit } : tab)),
    })),
}))
