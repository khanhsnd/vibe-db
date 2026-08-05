import type { ReactNode } from 'react'
import { Cable, Clock, Database, Rows3, Server } from 'lucide-react'

import { EngineIcon } from '@/features/connections/EngineIcon'
import { EnvironmentBadge } from '@/features/connections/EnvironmentDot'
import { StatusDot } from '@/features/connections/StatusDot'
import type { ConnectionStatus } from '@/features/connections/tree'
import { cn } from '@/lib/cn'

export interface StatusInfo {
  status: ConnectionStatus
  /** What the reader called this connection, which is what they know it by. */
  name: string | null
  target: string | null
  /** The engine of the connection being reported on, drawn as its chip. */
  engine: string | null
  /** What that connection is for, drawn as its badge. Empty for one that names none. */
  environment: string
  serverVersion: string | null
  /** Where the tab in front is pointed — `shop.public`. Empty where it names no namespace. */
  namespace: string
  /** How many connections are open at all, which is what says this bar reports on one of several. */
  connections: number
  rowCount: number | null
  elapsedMs: number | null
}

const numbers = new Intl.NumberFormat('en-US')

/**
 * What the tab in front is pointed at, read left to right in the order a reader asks it: is it
 * live, what kind of server, what is it *for*, which connection, which machine, which namespace,
 * which version. The right-hand end is what the app is holding rather than what this tab is —
 * how many connections are open — so the two are never told apart by position alone.
 */
export function StatusBar({
  status,
  name,
  target,
  engine,
  environment,
  serverVersion,
  namespace,
  connections,
  rowCount,
  elapsedMs,
}: StatusInfo) {
  return (
    <footer className="flex h-6 shrink-0 items-center gap-2 border-t border-line bg-panel px-2.5 text-xs text-muted">
      <StatusDot status={status} />
      {engine ? <EngineIcon engine={engine} className="h-3.5 w-3.5" /> : null}
      <EnvironmentBadge environment={environment} />

      {name ? <span className="shrink-0 truncate font-medium text-fg">{name}</span> : null}

      <Fact icon={<Server size={11} />}>{target ?? 'No connection'}</Fact>

      {namespace ? (
        <Fact icon={<Database size={11} />} tint="text-kind-database" bright>
          {namespace}
        </Fact>
      ) : null}

      {serverVersion ? <Fact>{serverVersion}</Fact> : null}

      <div className="min-w-2 flex-1" />

      {rowCount !== null ? (
        <Fact icon={<Rows3 size={11} />}>{`${numbers.format(rowCount)} rows`}</Fact>
      ) : null}
      {elapsedMs !== null ? (
        <Fact icon={<Clock size={11} />}>{`${numbers.format(elapsedMs)} ms`}</Fact>
      ) : null}

      {connections > 0 ? (
        <Fact icon={<Cable size={11} />} tint="text-success">
          {`${connections} open`}
        </Fact>
      ) : null}
    </footer>
  )
}

interface FactProps {
  /** The glyph in front, which is what the eye finds a fact by rather than by its position. */
  icon?: ReactNode
  /** The glyph's own colour, where the fact has one — a database's, a live connection's. */
  tint?: string
  /** Whether the value reads as bright as a heading. The namespace is; a version is not. */
  bright?: boolean
  children: ReactNode
}

/**
 * One fact of the bar: a glyph, its value, and the rule that divides it from the last. The value is
 * selectable — a host, a version, and, when nothing is connected, the wording of the failure that
 * is standing where the server should be.
 */
function Fact({ icon, tint, bright = false, children }: FactProps) {
  return (
    <span className="flex min-w-0 items-center gap-1 border-l border-line pl-2">
      {icon ? <span className={cn('shrink-0', tint)}>{icon}</span> : null}
      <span className={cn('selectable truncate font-mono', bright && 'text-fg')}>{children}</span>
    </span>
  )
}
