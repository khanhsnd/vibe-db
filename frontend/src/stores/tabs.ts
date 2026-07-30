import { create } from 'zustand'

import type { Namespace } from '@/bridge/connections'

import { useQueriesStore } from './queries'

export type TabKind = 'query' | 'table'

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

export type Tab = QueryTab | TableTab
export type NewTab = Omit<QueryTab, 'id'> | Omit<TableTab, 'id'>

type QueryEdit = Partial<Pick<QueryTab, 'sql' | 'namespace' | 'rowLimit'>>

interface TabsState {
  tabs: Tab[]
  activeId: string | null
  open: (tab: NewTab) => string
  close: (id: string) => void
  activate: (id: string) => void
  editQuery: (id: string, edit: QueryEdit) => void
}

let seq = 0
const nextId = () => `tab-${++seq}`

const sameTable = (tab: Tab, target: Omit<TableTab, 'id'>) =>
  tab.kind === 'table' &&
  tab.profileId === target.profileId &&
  tab.namespace.database === target.namespace.database &&
  tab.namespace.schema === target.namespace.schema &&
  tab.table === target.table

export const useTabsStore = create<TabsState>((set, get) => ({
  tabs: [],
  activeId: null,

  open: (tab) => {
    if (tab.kind === 'table') {
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

  editQuery: (id, edit) =>
    set((s) => ({
      tabs: s.tabs.map((tab) => (tab.id === id && tab.kind === 'query' ? { ...tab, ...edit } : tab)),
    })),
}))
