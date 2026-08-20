import { Database, Lock, TriangleAlert } from 'lucide-react'

import { EngineIcon } from '@/features/connections/EngineIcon'
import { EnvironmentBadge } from '@/features/connections/EnvironmentDot'
import { StatusDot } from '@/features/connections/StatusDot'
import { cn } from '@/lib/cn'

import type { StatusInfo } from './status'

/**
 * Which connection the reader is on, across the top of the window and in the middle of it, where
 * the eye lands before it lands on anything else. It is the same fact the bar along the bottom used
 * to carry alone, and it is drawn here instead because a reader who has not noticed it has not
 * noticed which server the next statement goes to — see `docs/features/shell.md`.
 *
 * The pill is drawn whether or not anything is connected: a place that empties is a place a reader
 * stops looking at, so an idle window says `No connection` in it and a window whose last attempt
 * failed says why.
 */
export function ConnectionBanner({
  status,
  name,
  target,
  engine,
  environment,
  readOnly,
  namespace,
}: StatusInfo) {
  const failed = status === 'error'

  return (
    <div
      className="flex h-7 min-w-0 max-w-[30rem] items-center gap-2 rounded-md border border-line bg-elevated px-2"
      title={target ?? undefined}
    >
      <StatusDot status={status} className="h-2.5 w-2.5" />
      {engine ? <EngineIcon engine={engine} /> : null}

      <span className="truncate text-sm font-semibold tracking-tight text-fg">
        {name ?? 'No connection'}
      </span>

      {target ? (
        <span
          className={cn(
            'selectable truncate font-mono text-2xs',
            failed ? 'text-danger' : 'text-muted',
          )}
        >
          {failed ? <TriangleAlert size={11} className="mr-1 inline align-[-1px]" /> : null}
          {target}
        </span>
      ) : null}

      {namespace ? (
        <span className="flex min-w-0 items-center gap-1 border-l border-line pl-2 text-kind-database">
          <Database size={12} className="shrink-0" />
          <span className="selectable truncate font-mono text-xs font-medium">{namespace}</span>
        </span>
      ) : null}

      {/* Beside the connection rather than in the bar below, because a reader who cannot see it
          finds out by being refused — see `docs/features/read-only.md`. */}
      {readOnly ? (
        <span className="flex shrink-0 items-center gap-1 rounded border border-warn px-1.5 py-px text-2xs font-medium uppercase text-warn">
          <Lock size={10} />
          Read-only
        </span>
      ) : null}

      <EnvironmentBadge environment={environment} />
    </div>
  )
}
