import { create } from 'zustand'

export type TabKind = 'query' | 'table'

export interface Tab {
  id: string
  kind: TabKind
  title: string
  sessionId: string | null
}

interface TabsState {
  tabs: Tab[]
  activeId: string | null
  open: (tab: Omit<Tab, 'id'>) => string
  close: (id: string) => void
  activate: (id: string) => void
}

let seq = 0
const nextId = () => `tab-${++seq}`

export const useTabsStore = create<TabsState>((set) => ({
  tabs: [],
  activeId: null,

  open: (tab) => {
    const id = nextId()
    set((s) => ({ tabs: [...s.tabs, { ...tab, id }], activeId: id }))
    return id
  },

  close: (id) =>
    set((s) => {
      const index = s.tabs.findIndex((t) => t.id === id)
      if (index === -1) return s

      const tabs = s.tabs.filter((t) => t.id !== id)
      if (s.activeId !== id) return { tabs, activeId: s.activeId }

      const neighbour = tabs[index] ?? tabs[index - 1]
      return { tabs, activeId: neighbour?.id ?? null }
    }),

  activate: (id) => set({ activeId: id }),
}))
