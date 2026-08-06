import { create } from 'zustand'

import type { CompareLayout } from '@/features/compare/ComparePanes'

interface UIState {
  sidebarCollapsed: boolean
  toggleSidebar: () => void
  /**
   * What the *splitter* found, which is the other way the sidebar closes: dragging its divider past
   * the width the panel is collapsible below. Without this the button and the panel disagree, and the
   * next press of the button does nothing the reader can see.
   */
  sidebarResized: (collapsed: boolean) => void
  /**
   * Which way round a comparison's two panes sit. It lives here rather than on a tab because it is
   * a preference about *reading* and not a fact about the pair: a reader who works on a wide screen
   * wants every comparison side by side, and picking it once per tab is picking it again and again.
   */
  compareLayout: CompareLayout
  setCompareLayout: (layout: CompareLayout) => void
}

export const useUIStore = create<UIState>((set) => ({
  sidebarCollapsed: false,
  compareLayout: 'columns',

  setCompareLayout: (compareLayout) => set({ compareLayout }),

  toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),

  sidebarResized: (collapsed) =>
    set((s) => (s.sidebarCollapsed === collapsed ? s : { sidebarCollapsed: collapsed })),
}))
