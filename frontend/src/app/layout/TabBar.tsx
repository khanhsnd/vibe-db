import { List, Plus, Table2, Terminal, X } from 'lucide-react'

import { environmentInfo, namespaceLabel, type Profile } from '@/bridge/connections'
import { EnvironmentStripe } from '@/features/connections/EnvironmentDot'
import { cn } from '@/lib/cn'
import { IconButton } from '@/shared/ui/IconButton'
import { useConnectionsStore } from '@/stores/connections'
import { useTabsStore, type Tab, type TabKind } from '@/stores/tabs'

const MIDDLE_BUTTON = 1

/** What a tab is showing, said in full — its title is only ever the last part of this. */
const KIND_LABEL: Record<TabKind, string> = {
  query: 'Query',
  table: 'Table',
  design: 'Design',
  objects: 'Tables',
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
  const lines = [named ? `${KIND_LABEL[tab.kind]}: ${tab.table}` : KIND_LABEL[tab.kind]]

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
  const Icon = tab.kind === 'query' ? Terminal : tab.kind === 'objects' ? List : Table2

  return (
    <div
      role="tab"
      aria-selected={active}
      title={tooltip}
      onClick={onSelect}
      onAuxClick={(e) => {
        if (e.button === MIDDLE_BUTTON) onClose()
      }}
      className={cn(
        'group relative flex max-w-[220px] min-w-0 shrink-0 cursor-default items-center gap-1.5',
        'border-r border-line px-3 text-sm whitespace-nowrap transition-colors',
        active ? 'bg-canvas text-fg' : 'text-muted hover:bg-hover hover:text-fg',
      )}
    >
      {active ? <span className="absolute inset-x-0 top-0 h-0.5 bg-accent" /> : null}
      <EnvironmentStripe environment={environment} />

      <Icon size={13} className="shrink-0 text-faint" />
      <span className="truncate">{tab.title}</span>

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
