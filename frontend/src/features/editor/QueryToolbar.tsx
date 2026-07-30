import { Database, ListTree, Loader2, Play, Square } from 'lucide-react'

import { QUERY_ROW_LIMITS } from '@/bridge/query'
import { Button } from '@/shared/ui/Button'

const SELECT_CLASS =
  'h-6 rounded-md border border-line bg-canvas px-1.5 text-xs text-fg outline-none focus:border-accent disabled:opacity-45'

interface QueryToolbarProps {
  databases: string[]
  database: string
  rowLimit: number
  running: boolean
  canRun: boolean
  onDatabaseChange: (database: string) => void
  onRowLimitChange: (limit: number) => void
  onExecute: () => void
  onExplain: () => void
  onStop: () => void
}

export function QueryToolbar({
  databases,
  database,
  rowLimit,
  running,
  canRun,
  onDatabaseChange,
  onRowLimitChange,
  onExecute,
  onExplain,
  onStop,
}: QueryToolbarProps) {
  return (
    <div
      role="toolbar"
      className="flex h-9 shrink-0 items-center gap-1.5 border-b border-line bg-panel px-2"
    >
      <Database size={13} className="shrink-0 text-faint" />
      <select
        aria-label="Database"
        value={database}
        disabled={databases.length === 0}
        onChange={(e) => onDatabaseChange(e.target.value)}
        className={`${SELECT_CLASS} max-w-[180px] font-mono`}
      >
        <option value="">{databases.length === 0 ? 'Not connected' : 'No database'}</option>
        {databases.map((name) => (
          <option key={name} value={name}>
            {name}
          </option>
        ))}
      </select>

      <span className="mx-1 h-4 w-px bg-line" />

      <Button
        variant="primary"
        icon={<Play size={12} />}
        disabled={!canRun || running}
        onClick={onExecute}
      >
        Execute
      </Button>
      <span className="mr-1 font-mono text-2xs text-faint">F5</span>

      <Button
        variant="default"
        icon={<ListTree size={12} />}
        disabled={!canRun || running}
        onClick={onExplain}
      >
        Explain
      </Button>

      <Button variant="default" icon={<Square size={11} />} disabled={!running} onClick={onStop}>
        Stop
      </Button>

      {running ? (
        <span className="ml-1 flex items-center gap-1.5 text-xs text-muted">
          <Loader2 size={13} className="animate-spin text-accent" />
          Running…
        </span>
      ) : null}

      <div className="min-w-2 flex-1" />

      <label className="flex shrink-0 items-center gap-1.5 text-xs text-muted">
        Limit
        <select
          value={rowLimit}
          onChange={(e) => onRowLimitChange(Number(e.target.value))}
          className={`${SELECT_CLASS} font-mono`}
        >
          {QUERY_ROW_LIMITS.map((limit) => (
            <option key={limit} value={limit}>
              {limit.toLocaleString('en-US')}
            </option>
          ))}
        </select>
      </label>
    </div>
  )
}
