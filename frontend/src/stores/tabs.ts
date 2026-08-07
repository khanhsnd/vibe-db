import { create } from 'zustand'

import { namespaceLabel, NO_NAMESPACE, type Namespace } from '@/bridge/connections'

import type { SearchMode, SearchScope } from '@/bridge/search'
import type { SourceKind, SourceRef } from '@/bridge/schema'

import { useConsoleStore } from './console'
import { useQueriesStore } from './queries'
import { useSearchesStore } from './searches'
import { useTableViewsStore } from './tableViews'

export type TabKind =
  | 'query'
  | 'table'
  | 'design'
  | 'source'
  | 'objects'
  | 'console'
  | 'compare'
  | 'find'
  | 'backups'
  | 'log'

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
 * A view's or a routine's **definition** rather than its rows — what Design View and Design
 * Function open, and what New View opens on the engine's own skeleton.
 *
 * The whole `CREATE` statement is the buffer, so there is no name box beside it: a definition
 * addresses the object it declares, and a definition is *one* statement however many semicolons a
 * routine body holds. Nothing on this side ever reads that text.
 */
export interface SourceTab extends TabBase {
  kind: 'source'
  ref: SourceRef
  /**
   * Whether the object is being **declared** rather than altered. It is a fact of its own rather
   * than a blank name, because a new one is named before its tab opens — the skeleton has to carry
   * the name — and what it decides is whether a Save drops anything first.
   */
  creating: boolean
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

/**
 * Every table of a namespace searched for one piece of text — what Find in Schema opens. The tab
 * holds the **question** and `stores/searches` holds the answer, exactly as a query tab keeps its
 * SQL while its rows live elsewhere: the question is what the tab is.
 *
 * Its connection and namespace are where it searches, and its own pickers are what move them —
 * which is why they are the tab's rather than the view's, the tab strip drawing both.
 */
export interface FindTab extends TabBase {
  kind: 'find'
  text: string
  scope: SearchScope
  mode: SearchMode
  caseInsensitive: boolean
}

/**
 * Every backup kept for a namespace — what the Backups node opens. It is the one tab in the strip
 * whose rows are **files on this machine** rather than anything a server answered with, and it
 * holds no state of its own: the folder on disk is the state, and the list is re-read rather than
 * remembered, a backup taken from another window being one this one has to show.
 */
export interface BackupsTab extends TabBase {
  kind: 'backups'
}

/**
 * Every statement the app has run since it started — what `Tools ▸ History Log` opens. It is the
 * one tab that belongs to no connection: what it shows is the *window's* doing, across all of them,
 * so its `profileId` and namespace are empty and the strip draws neither. There is one of it for
 * the same reason.
 */
export interface LogTab extends TabBase {
  kind: 'log'
}

export type Tab =
  | QueryTab
  | TableTab
  | DesignTab
  | SourceTab
  | ObjectsTab
  | ConsoleTab
  | CompareTab
  | FindTab
  | BackupsTab
  | LogTab
export type NewTab =
  | Omit<QueryTab, 'id'>
  | Omit<TableTab, 'id'>
  | Omit<DesignTab, 'id'>
  | Omit<SourceTab, 'id'>
  | Omit<ObjectsTab, 'id'>
  | Omit<ConsoleTab, 'id'>
  | Omit<CompareTab, 'id'>
  | Omit<FindTab, 'id'>
  | Omit<BackupsTab, 'id'>
  | Omit<LogTab, 'id'>

/** What names a table wherever one is opened: which connection, which namespace, which table. */
export interface TableTarget {
  profileId: string
  namespace: Namespace
  table: string
}

/** What names a definition wherever one is opened: a table target, plus what kind of thing it is. */
export interface SourceTarget {
  profileId: string
  namespace: Namespace
  ref: SourceRef
  /** True for one being declared rather than altered. See `SourceTab.creating`. */
  creating?: boolean
}

/** The word each kind reads as, wherever one is named to a person. */
export const SOURCE_NOUN: Record<SourceKind, string> = {
  view: 'View',
  function: 'Function',
  procedure: 'Procedure',
}

/** How a definition reads in the strip: the object's name, then what kind of thing it is. */
export function sourceTitle(ref: SourceRef): string {
  return `${ref.name} · ${SOURCE_NOUN[ref.kind]}`
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
type FindEdit = Partial<Pick<FindTab, 'text' | 'scope' | 'mode' | 'caseInsensitive'>>

interface TabsState {
  tabs: Tab[]
  activeId: string | null
  open: (tab: NewTab) => string
  /** The rows of a table, or of a view, which reads the same way. */
  openTable: (target: TableTarget) => string
  /** A table's shape. An empty `table` designs one that does not exist yet. */
  openDesign: (target: TableTarget) => string
  /** A view's or a routine's definition, read from the server or — with `creating` — declared. */
  openSource: (target: SourceTarget) => string
  /** Every table of a namespace, with what the catalogue keeps about each. */
  openTables: (profileId: string, namespace: Namespace) => string
  /** Every backup kept for a namespace. */
  openBackups: (profileId: string, namespace: Namespace) => string
  /** A console on one connection. One per connection and namespace, like every other tab. */
  openConsole: (profileId: string, namespace: Namespace) => string
  /**
   * Two tables side by side. One pair is one tab, so asking for the other half of a comparison
   * already open reveals it and switches its mode rather than opening a second tab on one pair.
   */
  openCompare: (left: TableTarget, right: TableTarget, mode: CompareMode) => string
  /** Switches an open comparison between the two halves — its toolbar's own control. */
  setCompareMode: (id: string, mode: CompareMode) => void
  /** Every table of a namespace searched for one piece of text. */
  openFind: (profileId: string, namespace: Namespace) => string
  /** Every statement the app has run. One tab, belonging to no connection — see `LogTab`. */
  openLog: () => string
  close: (id: string) => void
  activate: (id: string) => void
  /** Points a tab at another namespace, which is what the picker on its own strip does. */
  retarget: (id: string, namespace: Namespace) => void
  /**
   * Points a tab at another *connection* as well, which only Find in Schema's own pickers do: it
   * searches wherever the reader points it, and a namespace of one server names nothing on another.
   */
  repoint: (id: string, profileId: string, namespace: Namespace) => void
  editQuery: (id: string, edit: QueryEdit) => void
  editFind: (id: string, edit: FindEdit) => void
  designed: (id: string, table: string) => void
  /**
   * A definition tab whose Save has just declared its object now **edits** it, so pressing Save
   * again replaces it rather than declaring a second one and being refused for a name that is
   * taken. It is `designed`'s sibling; the object was named before the tab opened, so only which
   * of the two a Save is has changed.
   */
  declared: (id: string) => void
}

let seq = 0
const nextId = () => `tab-${++seq}`

// What a tab is *about*, for the one-tab-per-thing rule below. A definition is named by all three
// of its facts, because on PostgreSQL two functions of one name differ only in what they take —
// which is the same reason a drop carries the arguments.
const subjectOf = (tab: Tab | NewTab) => {
  if (tab.kind === 'source') return `${tab.ref.kind}:${tab.ref.name}(${tab.ref.arguments})`
  return 'table' in tab ? tab.table : ''
}

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
  subjectOf(tab) === subjectOf(target) &&
  (tab.kind !== 'compare' || sameTable(tab.right, (target as CompareTab).right))

// A query is never reused — two of them are two buffers — and neither is a table being designed
// that has no name yet, because two of those are two tables. An object being *declared* is the
// same case: it is not on the server for a second tab to be pointed at.
const reusable = (tab: NewTab) =>
  tab.kind !== 'query' &&
  !(tab.kind === 'design' && tab.table === '') &&
  !(tab.kind === 'source' && tab.creating)

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

  openSource: ({ profileId, namespace, ref, creating = false }) =>
    get().open({ kind: 'source', title: sourceTitle(ref), profileId, namespace, ref, creating }),

  // The namespace is not in the title: every tab in the strip draws the one it belongs to, and a
  // title that repeated it would say `hirec.public` twice on one tab.
  openTables: (profileId, namespace) =>
    get().open({ kind: 'objects', title: 'Tables', profileId, namespace }),

  openBackups: (profileId, namespace) =>
    get().open({ kind: 'backups', title: 'Backups', profileId, namespace }),

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

  // One search tab per namespace, like every other tab that names one: asking for it again on the
  // same namespace reveals the search already open there, text and hits included.
  openFind: (profileId, namespace) =>
    get().open({
      kind: 'find',
      title: 'Find in Schema',
      profileId,
      namespace,
      text: '',
      scope: 'data',
      mode: 'contains',
      caseInsensitive: false,
    }),

  openLog: () =>
    get().open({ kind: 'log', title: 'History Log', profileId: '', namespace: NO_NAMESPACE }),

  close: (id) => {
    useQueriesStore.getState().discard(id)
    useTableViewsStore.getState().discard(id)
    useConsoleStore.getState().discard(id)
    useSearchesStore.getState().discard(id)

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

  repoint: (id, profileId, namespace) =>
    set((s) => ({
      tabs: s.tabs.map((tab) => (tab.id === id ? { ...tab, profileId, namespace } : tab)),
    })),

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

  declared: (id) =>
    set((s) => ({
      tabs: s.tabs.map((tab) =>
        tab.id === id && tab.kind === 'source' ? { ...tab, creating: false } : tab,
      ),
    })),

  editQuery: (id, edit) =>
    set((s) => ({
      tabs: s.tabs.map((tab) => (tab.id === id && tab.kind === 'query' ? { ...tab, ...edit } : tab)),
    })),

  editFind: (id, edit) =>
    set((s) => ({
      tabs: s.tabs.map((tab) => (tab.id === id && tab.kind === 'find' ? { ...tab, ...edit } : tab)),
    })),
}))
