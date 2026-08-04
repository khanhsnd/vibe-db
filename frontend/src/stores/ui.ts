import { create } from 'zustand'

interface UIState {
  sidebarCollapsed: boolean
  toggleSidebar: () => void
  /**
   * What the *splitter* found, which is the other way the sidebar closes: dragging its divider past
   * the width the panel is collapsible below. Without this the button and the panel disagree, and the
   * next press of the button does nothing the reader can see.
   */
  sidebarResized: (collapsed: boolean) => void
}

export const useUIStore = create<UIState>((set) => ({
  sidebarCollapsed: false,

  toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),

  sidebarResized: (collapsed) =>
    set((s) => (s.sidebarCollapsed === collapsed ? s : { sidebarCollapsed: collapsed })),
}))
