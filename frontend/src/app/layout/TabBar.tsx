import {
  Archive,
  Columns3,
  FileCode2,
  GitCompareArrows,
  List,
  Plus,
  Search,
  SquareTerminal,
  Table2,
  Terminal,
  X,
  type LucideIcon,
} from 'lucide-react'

import { environmentInfo, namespaceLabel, type Profile } from '@/bridge/connections'
import { EnvironmentStripe } from '@/features/connections/EnvironmentDot'
import { cn } from '@/lib/cn'
import { IconButton } from '@/shared/ui/IconButton'
import { useConnectionsStore } from '@/stores/connections'
import { targetLabel, useTabsStore, type Tab, type TabKind } from '@/stores/tabs'

const MIDDLE_BUTTON = 1

/**
 * What a tab is, in one place: the word it is called by in full — its title is only ever the last
 * part of that — the glyph it wears, and the colour of that glyph. The colours are the tree's, so
 * a table opened from the sidebar is the same colour in the strip it lands in.
 */
export const KINDS: Record<TabKind, { label: string; icon: LucideIcon; tint: string }> = {
  query: { label: 'Query', icon: Terminal, tint: 'text-kind-routine' },
  table: { label: 'Table', icon: Table2, tint: 'text-kind-table' },
  design: { label: 'Design', icon: Columns3, tint: 'text-kind-field' },
  // A definition wears the view's colour whichever of the three it is: what the tab holds is a
  // *declaration*, and the title beside it already says which kind.
  source: { label: 'Definition', icon: FileCode2, tint: 'text-kind-view' },
  objects: { label: 'Tables', icon: List, tint: 'text-kind-database' },
  console: { label: 'Console', icon: SquareTerminal, tint: 'text-kind-routine' },
  compare: { label: 'Compare', icon: GitCompareArrows, tint: 'text-kind-schema' },
  find: { label: 'Find in Schema', icon: Search, tint: 'text-kind-database' },
  backups: { label: 'Backups', icon: Archive, tint: 'text-kind-backup' },
}

export function TabBar({ onNewQuery }: { onNewQuery: () => void }) {
  const tabs = useTabsStore((s) => s.tabs)
  const activeId = useTabsStore((s) => s.activeId)
  const activate = useTabsStore((s) => s.activate)
  const close = useTabsStore((s) => s.close)
  const profiles = useConnectionsStore((s) => s.profiles)

  return (
    <div
      role="tablist"
      className="scroll-thin flex h-8 shrink-0 items-stretch overflow-x-auto border-b border-line bg-panel"
    >
      {tabs.map((tab) => {
        const profile = profiles.find((one) => one.id === tab.profileId)

        return (
          <TabItem
            key={tab.id}
            tab={tab}
            active={tab.id === activeId}
            environment={profile?.environment ?? ''}
            tooltip={tabTooltip(tab, profile)}
            onSelect={() => activate(tab.id)}
            onClose={() => close(tab.id)}
          />
        )
      })}

      <div className="flex items-center px-1">
        <IconButton label="New query" icon={<Plus size={14} />} onClick={onNewQuery} />
      </div>
    </div>
  )
}

/**
 * Which table, which database, which server — a tab strip truncates to a name, and two tabs
 * called `users` on two connections are otherwise the same tab. It reads as lines rather than a
 * sentence because a reader is looking for one of the three facts, not reading prose.
 */
function tabTooltip(tab: Tab, profile: Profile | undefined): string {
  const named = 'table' in tab && tab.table !== ''
  const lines = [named ? `${KINDS[tab.kind].label}: ${tab.table}` : KINDS[tab.kind].label]

  // A comparison names two tables and the strip has room for neither in full, so the side that is
  // *not* the tab's own connection is spelled here — otherwise two tabs comparing `users` against
  // two different servers read as the same tab.
  if (tab.kind === 'compare') lines.push(`Against: ${targetLabel(tab.right)}`)

  if (tab.namespace.database !== '') lines.push(`Database: ${namespaceLabel(tab.namespace)}`)
  if (profile) {
    lines.push(`Connection: ${profile.name}`)
    lines.push(`Server: ${profile.user}@${profile.host}:${profile.port}`)
    if (profile.environment !== '') {
      lines.push(`Environment: ${environmentInfo(profile.environment).label}`)
    }
  }
  return lines.join('\n')
}

interface TabItemProps {
  tab: Tab
  active: boolean
  /** The connection's environment, drawn as the stripe down the tab's leading edge. */
  environment: string
  tooltip: string
  onSelect: () => void
  onClose: () => void
}

function TabItem({ tab, active, environment, tooltip, onSelect, onClose }: TabItemProps) {
  const { icon: Icon, tint } = KINDS[tab.kind]
  const where = namespaceLabel(tab.namespace)

  return (
    <div
      role="tab"
      aria-selected={active}
      title={tooltip}
      onClick={onSelect}
      // The wheel closes the tab, which is what it does to a tab everywhere. Chromium's own
      // middle-click autoscroll starts on the *press* and swallows the `auxclick` that would
      // follow, and this strip scrolls the moment it holds more tabs than it can draw — so
      // refusing the press is the whole of what keeps the gesture ours.
      onMouseDown={(e) => {
        if (e.button === MIDDLE_BUTTON) e.preventDefault()
      }}
      onAuxClick={(e) => {
        if (e.button === MIDDLE_BUTTON) onClose()
      }}
      className={cn(
        'group relative flex max-w-[260px] min-w-0 shrink-0 cursor-default items-center gap-1.5',
        'border-r border-line px-3 text-sm whitespace-nowrap transition-colors',
        active ? 'bg-canvas text-fg' : 'text-muted hover:bg-hover hover:text-fg',
      )}
    >
      {active ? <span className="absolute inset-x-0 top-0 h-0.5 bg-accent" /> : null}
      <EnvironmentStripe environment={environment} />

      {/* The tab's kind keeps its colour whether or not the tab is in front: it says what the tab
          *is*, which does not change with the selection, and a strip of grey glyphs would leave
          the reader reading every title to find the query among the tables. */}
      <Icon size={13} className={cn('shrink-0', tint)} />
      <span className="min-w-0 truncate">{tab.title}</span>

      {/* Which namespace, for the reader holding two tabs on one table name. It is capped well
          under the title's share of the tab: the title is what was asked for, and this only says
          which of two the reader is looking at. */}
      {where === '' ? null : (
        <span className="max-w-[84px] shrink-0 truncate font-mono text-2xs text-faint">
          {where}
        </span>
      )}

      <button
        type="button"
        aria-label={`Close ${tab.title}`}
        onClick={(e) => {
          e.stopPropagation()
          onClose()
        }}
        className={cn(
          'ml-0.5 grid h-4 w-4 shrink-0 place-items-center rounded',
          'text-faint opacity-0 hover:bg-hover hover:text-fg',
          'group-hover:opacity-100 focus-visible:opacity-100',
          active && 'opacity-60',
        )}
      >
        <X size={11} />
      </button>
    </div>
  )
}
