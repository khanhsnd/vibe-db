import { Moon, PanelLeft, Plus, Settings, Sun, Terminal } from 'lucide-react'

import { IconButton } from '@/shared/ui/IconButton'
import { Button } from '@/shared/ui/Button'
import { useTheme } from '@/shared/theme/useTheme'
import { useUIStore } from '@/stores/ui'

interface CommandBarProps {
  onNewConnection: () => void
  onNewQuery: () => void
}

export function CommandBar({ onNewConnection, onNewQuery }: CommandBarProps) {
  const { theme, toggleTheme } = useTheme()
  const toggleSidebar = useUIStore((s) => s.toggleSidebar)

  return (
    <header className="flex h-9 shrink-0 items-center gap-2 border-b border-line bg-panel px-2">
      <IconButton
        label="Toggle sidebar"
        icon={<PanelLeft size={15} />}
        onClick={toggleSidebar}
      />

      <span className="text-sm font-semibold tracking-tight text-fg">vibe-db</span>

      <span className="mx-1 h-4 w-px bg-line" />

      <Button variant="subtle" icon={<Plus size={14} />} onClick={onNewConnection}>
        New Connection
      </Button>

      <Button variant="subtle" icon={<Terminal size={14} />} onClick={onNewQuery}>
        New Query
      </Button>

      <div className="flex-1" />

      <IconButton
        label={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
        icon={theme === 'dark' ? <Sun size={15} /> : <Moon size={15} />}
        onClick={toggleTheme}
      />
      <IconButton label="Settings" icon={<Settings size={15} />} />
    </header>
  )
}
