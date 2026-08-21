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
import { MenuButton } from '@/shared/ui/MenuButton'
import { ICON } from '@/shared/ui/menuIcons'
import { useTheme } from '@/shared/theme/useTheme'
import { useConnectionsStore } from '@/stores/connections'
import { useTabsStore } from '@/stores/tabs'
import { useUIStore } from '@/stores/ui'

import { ConnectionBanner } from './ConnectionBanner'
import { newConnectionItems } from './newMenu'
import type { StatusInfo } from './status'

interface CommandBarProps {
  onNewConnection: () => void
  /**
   * A connection to an **object store**, which is the second kind of thing this app can be pointed
   * at. It is a menu beside the first and not a second button — see `newConnectionItems`.
   */
  onNewStore: () => void
  onNewQuery: () => void
  /**
   * Makes a bucket on the object store the reader is looking at, and `null` everywhere else — which
   * is what **replaces** New Query rather than sitting beside it.
   *
   * They are one slot because they are the same sentence about two kinds of connection: *make me a
   * new place to work*. A bucket has no statements, so a New Query button over one is a button that
   * opens a tab which can never run anything; and a database has no buckets. Drawing both would be
   * drawing one dead control at all times.
   */
  onNewBucket: (() => void) | null
  /**
   * Takes a backup of wherever the reader is. It is `null` — and the button dead — where there is
   * nothing to back up: no live connection, one whose engine has no such thing, or an object store,
   * which has no namespace to back up at all. The bar does not work that out for itself, because
   * which namespace it would be *of* is the shell's answer and not this strip's.
   */
  onBackup: (() => void) | null
  /**
   * Which connection the tab in front is on, worked out by the shell for the same reason `onBackup`
   * is: it is a fact about the window and not about this strip.
   */
  status: StatusInfo
}

export function CommandBar({
  onNewConnection,
  onNewStore,
  onNewQuery,
  onNewBucket,
  onBackup,
  status,
}: CommandBarProps) {
  const { theme, toggleTheme } = useTheme()
  const toggleSidebar = useUIStore((s) => s.toggleSidebar)
  // The one action here that is nobody's target: a job names the connection it dials, so this asks
  // the shell for nothing and is never dead — which is why it comes from the store rather than
  // being handed down as the other three are.
  const openAutomation = useTabsStore((s) => s.openAutomation)
  const [options, setOptions] = useState(false)

  const open = useConnectionsStore((s) => Object.keys(s.sessions).length)
  const saved = useConnectionsStore((s) => s.profiles.length)

  // Three columns rather than one row of controls, so the banner sits in the middle of the window
  // and stays there as the actions on either side of it come and go. The two `1fr` ends are what
  // keep it off them: an element merely centred over the strip would ride over the buttons the
  // moment the window narrowed. Only the banner carries `min-w-0`, so a narrow window shortens
  // the host it names rather than the labels on the buttons.
  return (
    <header className="grid h-11 shrink-0 grid-cols-[1fr_auto_1fr] items-center gap-2 border-b border-line bg-panel px-2">
      <div className="flex items-center gap-2">
        <IconButton label="Toggle sidebar" icon={<PanelLeft size={15} />} onClick={toggleSidebar} />

        {/* The app's own mark, in the accent: the one thing on the bar that is not a control, and
            what tells the reader at a glance which window of several this is. */}
        <span className="flex min-w-0 items-center gap-1.5">
          <span className="grid h-5 w-5 shrink-0 place-items-center rounded-md bg-accent text-on-accent">
            <Database size={12} strokeWidth={2.5} />
          </span>
          <span className="text-sm font-semibold tracking-tight text-fg">Vibe DB</span>
        </span>

        <span className="mx-1 h-4 w-px bg-line" />

        {/* The button a reader actually reaches for, so it is where **both** kinds are offered: a
            store reachable only from the sidebar's 24-pixel `+` is a store nobody finds. */}
        <MenuButton items={newConnectionItems(onNewConnection, onNewStore)}>
          <Button variant="subtle" icon={<Plus size={14} />}>
            New Connection
          </Button>
        </MenuButton>

        {/* One slot, two sentences — see `onNewBucket`. What the reader is looking at decides which,
            so neither is ever drawn dead. */}
        {onNewBucket !== null ? (
          <Button variant="subtle" icon={ICON.bucket} onClick={onNewBucket}>
            New Bucket
          </Button>
        ) : (
          <Button variant="subtle" icon={<Terminal size={14} />} onClick={onNewQuery}>
            New Query
          </Button>
        )}

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
      </div>

      <ConnectionBanner {...status} />

      <div className="flex items-center justify-end gap-2">
        {/* How much the app is holding, where a reader looks before deciding whether to open more:
            how many of the saved connections are live. Nothing at all is saved on a first run, and
            the count would then say only that — so it is not drawn. */}
        {saved > 0 ? (
          <span
            title={`${open} of ${saved} connections open`}
            className="shrink-0 font-mono text-2xs text-muted tabular-nums"
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
      </div>

      <SettingsDialog open={options} onOpenChange={setOptions} />
    </header>
  )
}
