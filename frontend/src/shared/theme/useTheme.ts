import { useSettingsStore } from '@/stores/settings'

export function useTheme() {
  const theme = useSettingsStore((s) => s.settings.theme)
  const update = useSettingsStore((s) => s.update)

  return {
    theme,
    toggleTheme: () => update({ theme: theme === 'dark' ? 'light' : 'dark' }),
  }
}
