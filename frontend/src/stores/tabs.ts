import { create } from 'zustand'

import type { Namespace } from '@/bridge/connections'

import { useQueriesStore } from './queries'

export type TabKind = 'query' | 'table' | 'design'

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

export type Tab = QueryTab | TableTab | DesignTab
export type NewTab = Omit<QueryTab, 'id'> | Omit<TableTab, 'id'> | Omit<DesignTab, 'id'>

type QueryEdit = Partial<Pick<QueryTab, 'sql' | 'namespace' | 'rowLimit'>>

interface TabsState {
  tabs: Tab[]
  activeId: string | null
  open: (tab: NewTab) => string
  close: (id: string) => void
  activate: (id: string) => void
  editQuery: (id: string, edit: QueryEdit) => void
  designed: (id: string, table: string) => void
}

let seq = 0
const nextId = () => `tab-${++seq}`

// One tab per table per kind: opening a table twice reveals the tab already showing it, and its
// data tab and its designer are two different tabs of the same table.
const sameTable = (tab: Tab, target: Omit<TableTab | DesignTab, 'id'>) =>
  tab.kind === target.kind &&
  tab.profileId === target.profileId &&
  tab.namespace.database === target.namespace.database &&
  tab.namespace.schema === target.namespace.schema &&
  'table' in tab &&
  tab.table === target.table

export const useTabsStore = create<TabsState>((set, get) => ({
  tabs: [],
  activeId: null,

  open: (tab) => {
    // A table being designed that has no name yet is never reused: two of them are two tables.
    if (tab.kind === 'table' || (tab.kind === 'design' && tab.table !== '')) {
      const existing = get().tabs.find((current) => sameTable(current, tab))
      if (existing) {
        set({ activeId: existing.id })
        return existing.id
      }
    }

    const id = nextId()
    set((s) => ({ tabs: [...s.tabs, { ...tab, id }], activeId: id }))
    return id
  },

  close: (id) => {
    useQueriesStore.getState().discard(id)

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
