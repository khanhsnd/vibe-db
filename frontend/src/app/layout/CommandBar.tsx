import { useState } from 'react'
import {
  Archive,
  CalendarClock,
  Database,
  Moon,
  PanelLeft,
  Plus,
  Settings,
  Sun,
  Terminal,
} from 'lucide-react'

import { SettingsDialog } from '@/features/settings/SettingsDialog'
import { IconButton } from '@/shared/ui/IconButton'
import { Button } from '@/shared/ui/Button'
import { useTheme } from '@/shared/theme/useTheme'
import { useConnectionsStore } from '@/stores/connections'
import { useTabsStore } from '@/stores/tabs'
import { useUIStore } from '@/stores/ui'

interface CommandBarProps {
  onNewConnection: () => void
  onNewQuery: () => void
  /**
   * Takes a backup of wherever the reader is. It is `null` — and the button dead — where there is
   * nothing to back up: no live connection, or one whose engine has no such thing. The bar does not
   * work that out for itself, because which namespace it would be *of* is the shell's answer and
   * not this strip's.
   */
  onBackup: (() => void) | null
}

export function CommandBar({ onNewConnection, onNewQuery, onBackup }: CommandBarProps) {
  const { theme, toggleTheme } = useTheme()
  const toggleSidebar = useUIStore((s) => s.toggleSidebar)
  // The one action here that is nobody's target: a job names the connection it dials, so this asks
  // the shell for nothing and is never dead — which is why it comes from the store rather than
  // being handed down as the other three are.
  const openAutomation = useTabsStore((s) => s.openAutomation)
  const [options, setOptions] = useState(false)

  const open = useConnectionsStore((s) => Object.keys(s.sessions).length)
  const saved = useConnectionsStore((s) => s.profiles.length)

  return (
    <header className="flex h-9 shrink-0 items-center gap-2 border-b border-line bg-panel px-2">
      <IconButton label="Toggle sidebar" icon={<PanelLeft size={15} />} onClick={toggleSidebar} />

      {/* The app's own mark, in the accent: the one thing on the bar that is not a control, and
          what tells the reader at a glance which window of several this is. */}
      <span className="flex min-w-0 items-center gap-1.5">
        <span className="grid h-5 w-5 shrink-0 place-items-center rounded-md bg-accent text-on-accent">
          <Database size={12} strokeWidth={2.5} />
        </span>
        <span className="text-sm font-semibold tracking-tight text-fg">vibe-db</span>
      </span>

      <span className="mx-1 h-4 w-px bg-line" />

      <Button variant="subtle" icon={<Plus size={14} />} onClick={onNewConnection}>
        New Connection
      </Button>

      <Button variant="subtle" icon={<Terminal size={14} />} onClick={onNewQuery}>
        New Query
      </Button>

      <Button
        variant="subtle"
        icon={<Archive size={14} />}
        disabled={onBackup === null}
        onClick={() => onBackup?.()}
      >
        Backup
      </Button>

      <Button variant="subtle" icon={<CalendarClock size={14} />} onClick={openAutomation}>
        Automation
      </Button>

      <div className="flex-1" />

      {/* How much the app is holding, where a reader looks before deciding whether to open more:
          how many of the saved connections are live. Nothing at all is saved on a first run, and
          the count would then say only that — so it is not drawn. */}
      {saved > 0 ? (
        <span
          title={`${open} of ${saved} connections open`}
          className="mr-1 shrink-0 font-mono text-2xs text-muted tabular-nums"
        >
          <span className={open > 0 ? 'text-success' : 'text-faint'}>{open}</span>
          <span className="text-faint">{` / ${saved}`}</span>
        </span>
      ) : null}

      <IconButton
        label={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
        icon={theme === 'dark' ? <Sun size={15} /> : <Moon size={15} />}
        onClick={toggleTheme}
      />
      <IconButton label="Options" icon={<Settings size={15} />} onClick={() => setOptions(true)} />

      <SettingsDialog open={options} onOpenChange={setOptions} />
    </header>
  )
}
