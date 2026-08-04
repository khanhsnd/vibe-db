import { Boxes, Database, ListTree, Loader2, Play, Square, WrapText } from 'lucide-react'

import type { Namespace } from '@/bridge/connections'
import { DEFAULT_QUERY_ROWS, QUERY_ROW_LIMITS } from '@/bridge/rows'
import { Button } from '@/shared/ui/Button'
import { TOOLBAR_CONTROL } from '@/shared/ui/Field'
import { RowLimit } from '@/shared/ui/RowLimit'

interface QueryToolbarProps {
  databases: string[]
  schemas: string[]
  namespace: Namespace
  /** Whether this engine has a schema below a database, and so a second picker. */
  hasSchemas: boolean
  rowLimit: number
  running: boolean
  canRun: boolean
  onDatabaseChange: (database: string) => void
  onSchemaChange: (schema: string) => void
  onRowLimitChange: (limit: number) => void
  onExecute: () => void
  onExplain: () => void
  onFormat: () => void
  onStop: () => void
}

export function QueryToolbar({
  databases,
  schemas,
  namespace,
  hasSchemas,
  rowLimit,
  running,
  canRun,
  onDatabaseChange,
  onSchemaChange,
  onRowLimitChange,
  onExecute,
  onExplain,
  onFormat,
  onStop,
}: QueryToolbarProps) {
  return (
    <div
      role="toolbar"
      className="flex h-9 shrink-0 items-center gap-1.5 border-b border-line bg-bar px-2"
    >
      <Database size={13} className="shrink-0 text-faint" />
      <NamespaceSelect
        label="Database"
        value={namespace.database}
        options={databases}
        empty={databases.length === 0 ? 'Not connected' : 'No database'}
        onChange={onDatabaseChange}
      />

      {hasSchemas ? (
        <>
          <Boxes size={13} className="shrink-0 text-faint" />
          <NamespaceSelect
            label="Schema"
            value={namespace.schema}
            options={schemas}
            empty={namespace.database === '' ? 'No database' : 'No schema'}
            onChange={onSchemaChange}
          />
        </>
      ) : null}

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

      <span className="mx-1 h-4 w-px bg-line" />

      {/* Laying a script out is the engine's — where a statement ends and what a quote is are
          lexical — so it needs the connection exactly as Execute does. */}
      <Button
        variant="default"
        icon={<WrapText size={12} />}
        disabled={!canRun || running}
        onClick={onFormat}
      >
        Format
      </Button>
      <span className="mr-1 font-mono text-2xs text-faint">Ctrl+Shift+F</span>

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

interface NamespaceSelectProps {
  label: string
  value: string
  options: string[]
  empty: string
  onChange: (value: string) => void
}

function NamespaceSelect({ label, value, options, empty, onChange }: NamespaceSelectProps) {
  return (
    <select
      aria-label={label}
      value={value}
      disabled={options.length === 0}
      onChange={(e) => onChange(e.target.value)}
      className={`${TOOLBAR_CONTROL} max-w-[180px] font-mono`}
    >
      <option value="">{empty}</option>
      {options.map((name) => (
        <option key={name} value={name}>
          {name}
        </option>
      ))}
    </select>
  )
}
