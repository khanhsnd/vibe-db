import { create } from 'zustand'

import { DEFAULT_PAGE_SIZE, DEFAULT_QUERY_ROWS } from '@/bridge/rows'
import {
  applyDataFont,
  applyTheme,
  DEFAULT_DATA_FONT,
  type DataFont,
  type Theme,
} from '@/shared/theme/appearance'

/** Every preference the reader can set, which is the whole of what the Options dialog edits. */
export interface Settings {
  theme: Theme
  font: DataFont
  /** Whether an over-long SQL line is wrapped or scrolled to. */
  wordWrap: boolean
  /** How many rows a page holds when a tab opens — a table read and a query result alike. */
  pageSize: number
  /** The cap a new query tab starts with. */
  queryRows: number
}

export const DEFAULT_SETTINGS: Settings = {
  theme: 'dark',
  font: DEFAULT_DATA_FONT,
  wordWrap: true,
  pageSize: DEFAULT_PAGE_SIZE,
  queryRows: DEFAULT_QUERY_ROWS,
}

const SETTINGS_KEY = 'vibe-db.settings'
/** The theme had a key of its own before Settings existed; it stands as the default until one is saved. */
const THEME_KEY = 'vibe-db.theme'

function stored(): Settings {
  const legacy: Settings = {
    ...DEFAULT_SETTINGS,
    theme: localStorage.getItem(THEME_KEY) === 'light' ? 'light' : DEFAULT_SETTINGS.theme,
  }

  const raw = localStorage.getItem(SETTINGS_KEY)
  if (raw === null) return legacy

  try {
    const parsed = JSON.parse(raw) as Partial<Settings>
    return { ...legacy, ...parsed, font: { ...legacy.font, ...parsed.font } }
  } catch {
    return legacy
  }
}

/** Saves the settings and puts them in force, which is the only way either ever happens. */
function commit(settings: Settings): Settings {
  localStorage.setItem(SETTINGS_KEY, JSON.stringify(settings))
  applyTheme(settings.theme)
  applyDataFont(settings.font)
  return settings
}

interface SettingsState {
  settings: Settings
  update: (part: Partial<Settings>) => void
  restoreDefaults: () => void
}

export const useSettingsStore = create<SettingsState>((set, get) => ({
  settings: stored(),

  update: (part) => set({ settings: commit({ ...get().settings, ...part }) }),
  restoreDefaults: () => set({ settings: commit(DEFAULT_SETTINGS) }),
}))

applyTheme(useSettingsStore.getState().settings.theme)
applyDataFont(useSettingsStore.getState().settings.font)
