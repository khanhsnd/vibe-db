import { EngineIcon } from '@/features/connections/EngineIcon'
import { EnvironmentBadge } from '@/features/connections/EnvironmentDot'
import { StatusDot } from '@/features/connections/StatusDot'
import type { ConnectionStatus } from '@/features/connections/tree'

export interface StatusInfo {
  status: ConnectionStatus
  target: string | null
  /** The engine of the connection being reported on, drawn as its chip. */
  engine: string | null
  /** What that connection is for, drawn as its badge. Empty for one that names none. */
  environment: string
  serverVersion: string | null
  rowCount: number | null
  elapsedMs: number | null
}

const numbers = new Intl.NumberFormat('en-US')

export function StatusBar({
  status,
  target,
  engine,
  environment,
  serverVersion,
  rowCount,
  elapsedMs,
}: StatusInfo) {
  return (
    <footer className="flex h-6 shrink-0 items-center gap-2 border-t border-line bg-panel px-2.5 text-xs text-muted">
      <StatusDot status={status} />
      {engine ? <EngineIcon engine={engine} className="h-3.5 w-3.5" /> : null}
      <EnvironmentBadge environment={environment} />
      <span className="truncate">{target ?? 'No connection'}</span>

      {serverVersion ? (
        <>
          <Separator />
          <span className="truncate">{serverVersion}</span>
        </>
      ) : null}

      <div className="flex-1" />

      {rowCount !== null ? (
        <span className="font-mono">{numbers.format(rowCount)} rows</span>
      ) : null}
      {elapsedMs !== null ? (
        <>
          <Separator />
          <span className="font-mono">{elapsedMs} ms</span>
        </>
      ) : null}
    </footer>
  )
}

function Separator() {
  return <span className="text-faint">·</span>
}
