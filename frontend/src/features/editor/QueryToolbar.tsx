import { Database, ListTree, Loader2, Play, Square } from 'lucide-react'

import { DEFAULT_QUERY_ROWS, QUERY_ROW_LIMITS } from '@/bridge/rows'
import { Button } from '@/shared/ui/Button'
import { TOOLBAR_CONTROL } from '@/shared/ui/Field'
import { RowLimit } from '@/shared/ui/RowLimit'

interface QueryToolbarProps {
  databases: string[]
  database: string
  // What the engine calls the namespace on offer — a database, or a PostgreSQL schema.
  namespace: string
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
  namespace,
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
        aria-label={namespace}
        value={database}
        disabled={databases.length === 0}
        onChange={(e) => onDatabaseChange(e.target.value)}
        className={`${TOOLBAR_CONTROL} max-w-[180px] font-mono`}
      >
        <option value="">
          {databases.length === 0 ? 'Not connected' : `No ${namespace.toLowerCase()}`}
        </option>
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

      <div className="flex shrink-0 items-center text-xs text-muted">
        <RowLimit
          label="Limit"
          presets={QUERY_ROW_LIMITS}
          fallback={DEFAULT_QUERY_ROWS}
          value={rowLimit}
          onChange={onRowLimitChange}
        />
      </div>
    </div>
  )
}
