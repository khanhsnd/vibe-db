import type { ReactNode } from 'react'
import { Cable, Clock, Rows3, Server, TriangleAlert } from 'lucide-react'

import { cn } from '@/lib/cn'

import type { StatusInfo } from './status'

const numbers = new Intl.NumberFormat('en-US')

/**
 * What the connection in front has *cost*: which version of the server answered, how many rows the
 * last read came to and how long it took — and at the right-hand end what the app is holding rather
 * than what this tab is, how many connections are open.
 *
 * *Which* connection it is belongs to `ConnectionBanner` and is not repeated here: a fact drawn
 * twice is a fact that will disagree with itself, and this strip is too small to be the only place
 * a reader is told which server they are about to write to.
 */
export function StatusBar({
  serverVersion,
  connections,
  rowCount,
  elapsedMs,
  error,
}: StatusInfo) {
  return (
    <footer className="flex h-6 shrink-0 items-center gap-2 border-t border-line bg-panel px-2.5 text-xs text-muted">
      {serverVersion ? (
        <Fact icon={<Server size={11} />} plain>
          {serverVersion}
        </Fact>
      ) : null}

      <div className="min-w-2 flex-1" />

      {error ? (
        <Fact icon={<TriangleAlert size={11} />} tint="text-danger">
          {error}
        </Fact>
      ) : null}

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
  /** The glyph's own colour, where the fact has one — a live connection's. */
  tint?: string
  /** Whether the fact stands first on the bar, and so has no rule dividing it from a last one. */
  plain?: boolean
  children: ReactNode
}

/**
 * One fact of the bar: a glyph, its value, and the rule that divides it from the last. The value is
 * selectable — a host, a version, and the wording of the failure nothing else was waiting on.
 */
function Fact({ icon, tint, plain = false, children }: FactProps) {
  return (
    <span className={cn('flex min-w-0 items-center gap-1', !plain && 'border-l border-line pl-2')}>
      {icon ? <span className={cn('shrink-0', tint)}>{icon}</span> : null}
      <span className="selectable truncate font-mono">{children}</span>
    </span>
  )
}
