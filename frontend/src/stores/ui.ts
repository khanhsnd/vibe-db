import { create } from 'zustand'

import { applyTheme } from '@/shared/theme/useTheme'

type Theme = 'dark' | 'light'

const THEME_KEY = 'vibe-db.theme'

function storedTheme(): Theme {
  return localStorage.getItem(THEME_KEY) === 'light' ? 'light' : 'dark'
}

interface UIState {
  theme: Theme
  sidebarCollapsed: boolean
  toggleTheme: () => void
  toggleSidebar: () => void
}

export const useUIStore = create<UIState>((set, get) => ({
  theme: storedTheme(),
  sidebarCollapsed: false,

  toggleTheme: () => {
    const theme: Theme = get().theme === 'dark' ? 'light' : 'dark'
    localStorage.setItem(THEME_KEY, theme)
    applyTheme(theme)
    set({ theme })
  },

  toggleSidebar: () => set((s) => ({ sidebarCollapsed: !s.sidebarCollapsed })),
}))

applyTheme(useUIStore.getState().theme)
