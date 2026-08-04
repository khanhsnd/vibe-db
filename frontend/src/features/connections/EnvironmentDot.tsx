import { environmentInfo } from '@/bridge/connections'
import { cn } from '@/lib/cn'

/**
 * The colour a connection's environment is drawn in, wherever that connection's name appears: the
 * tree, the tab strip, the status bar and the dialog that names it. It is one component so all
 * four say the same thing in the same colour — and it is the *only* thing that draws an
 * environment, so adding one is a row in `ENVIRONMENTS` and nothing else.
 *
 * A connection that names no environment still draws a dot, in the muted colour, because a gap
 * where the other rows have a mark reads as a rendering fault rather than as an absence.
 */
export function EnvironmentDot({
  environment,
  className,
}: {
  environment: string
  className?: string
}) {
  const info = environmentInfo(environment)

  return (
    <span
      role="img"
      aria-label={`Environment: ${info.label}`}
      title={`Environment: ${info.label}`}
      style={{ backgroundColor: info.colour }}
      className={cn('h-2 w-2 shrink-0 rounded-full', className)}
    />
  )
}

/**
 * The same fact as a strip down the left edge of a row, for the surfaces that carry a whole line
 * of the connection rather than a name in a sentence — a tree row, a tab. It is two pixels and
 * unmissable, which is the point: a reader glancing at a tab strip should see *where* they are
 * before they read *what* is open.
 */
export function EnvironmentStripe({ environment }: { environment: string }) {
  const info = environmentInfo(environment)
  if (info.value === '') return null

  return (
    <span
      aria-hidden
      style={{ backgroundColor: info.colour }}
      className="absolute inset-y-0 left-0 w-0.5"
    />
  )
}

/**
 * The environment written out, for a surface with room for a word: the dialog that confirms a
 * dangerous statement, and the status bar. Production is the one drawn filled, because it is the
 * one whose whole purpose is to be noticed.
 */
export function EnvironmentBadge({ environment }: { environment: string }) {
  const info = environmentInfo(environment)
  if (info.value === '') return null

  return (
    <span
      style={
        info.guarded
          ? { backgroundColor: info.colour, color: 'var(--fg-on-accent)' }
          : { color: info.colour, borderColor: info.colour }
      }
      className={cn(
        'shrink-0 rounded px-1.5 py-px text-2xs font-medium uppercase',
        info.guarded ? '' : 'border',
      )}
    >
      {info.label}
    </span>
  )
}
