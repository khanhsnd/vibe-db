import { Check } from 'lucide-react'

import { namespaceLabel, type Profile } from '@/bridge/connections'
import type { MenuItem } from '@/shared/ui/ContextMenu'
import { MenuButton } from '@/shared/ui/MenuButton'
import { useConnectionsStore } from '@/stores/connections'
import { useTabsStore, type Tab } from '@/stores/tabs'

import { KINDS } from './TabBar'

const MENUS = ['File', 'Edit', 'View', 'Favorites', 'Tools', 'Window', 'Help']

export function MenuBar() {
  const tabs = useTabsStore((s) => s.tabs)
  const activeId = useTabsStore((s) => s.activeId)
  const activate = useTabsStore((s) => s.activate)
  const profiles = useConnectionsStore((s) => s.profiles)

  // A menu with no items is a name that does nothing, which is what the other six still are — see
  // `MenuButton`, which draws its trigger alone when it is given none.
  const menus: Partial<Record<string, MenuItem[]>> = {
    Window: windowItems(tabs, activeId, profiles, activate),
  }

  return (
    <nav className="flex h-7 shrink-0 items-center gap-0.5 border-b border-line bg-panel px-1.5">
      {MENUS.map((label) => (
        <MenuButton key={label} items={menus[label] ?? []}>
          <button
            type="button"
            className="rounded px-2 py-0.5 text-xs text-fg transition-colors hover:bg-hover data-[state=open]:bg-hover"
          >
            {label}
          </button>
        </MenuButton>
      ))}
    </nav>
  )
}

/**
 * Every tab the window holds, in the order the strip draws them — what `Window` opens, and the one
 * place that names all of them at once: the strip truncates each title and scrolls the moment it
 * holds more tabs than it can draw. Picking one is exactly what clicking it in the strip is.
 */
function windowItems(
  tabs: Tab[],
  activeId: string | null,
  profiles: Profile[],
  activate: (id: string) => void,
): MenuItem[] {
  if (tabs.length === 0) return [{ label: 'No open tabs', disabled: true }]

  return tabs.map((tab) => {
    const { icon: Icon, tint } = KINDS[tab.kind]

    return {
      label: windowLabel(tab, profiles.find((one) => one.id === tab.profileId)),
      // The tab in front wears a tick where its kind's glyph goes, the way a window menu does
      // everywhere: its kind is still drawn on the tab itself, and which one the reader is on is
      // the one fact the strip stops telling them once it has scrolled.
      icon:
        tab.id === activeId ? (
          <Check size={13} className="text-accent" />
        ) : (
          <Icon size={13} className={tint} />
        ),
      onSelect: () => activate(tab.id),
    }
  })
}

/**
 * What a tab is called where there is room for it: its title, then where it is. A strip has room
 * for the title alone, so two tabs called `users` on two servers read there as one tab.
 */
function windowLabel(tab: Tab, profile: Profile | undefined): string {
  const where = [namespaceLabel(tab.namespace), profile?.name ?? ''].filter((part) => part !== '')
  return where.length === 0 ? tab.title : `${tab.title} — ${where.join(' · ')}`
}
