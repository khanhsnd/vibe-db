import { useUIStore } from '@/stores/ui'

export function applyTheme(theme: 'dark' | 'light') {
  document.documentElement.dataset.theme = theme
}

export function useTheme() {
  const theme = useUIStore((s) => s.theme)
  const toggleTheme = useUIStore((s) => s.toggleTheme)
  return { theme, toggleTheme }
}
