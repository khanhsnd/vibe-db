import { Loader2 } from 'lucide-react'

/**
 * A read the reader is waiting on, drawn where its rows will land. It fills the space the way
 * `Empty` does, because that is the same claim about the same area — one says there is nothing
 * to show, the other that there is not yet. A grid with no columns yet would otherwise read as
 * a blank pane, and a big table can take seconds to answer.
 */
export function Loading({ label = 'Loading…' }: { label?: string }) {
  return (
    <div className="grid h-full place-items-center p-8">
      <span className="flex items-center gap-2 text-sm text-muted">
        <Loader2 size={14} className="shrink-0 animate-spin text-accent" />
        {label}
      </span>
    </div>
  )
}
