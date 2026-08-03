import { useEffect, useState } from 'react'
import { Loader2 } from 'lucide-react'

import { Button } from './Button'

/** Often enough that the clock reads as running, rarely enough to cost nothing. */
const TICK_MS = 100

const numbers = new Intl.NumberFormat('en-US')

interface FetchProgressProps {
  fetched: number
  onStop: () => void
}

/**
 * A read that may take minutes, reported while it runs: how long it has been going, how many
 * rows have arrived, and the one control that matters — Stop. Where `Loading` says a pane has
 * nothing *yet* and fills it, this says a read is still arriving and sits over what is already
 * there, because the rows on screen are still the rows on screen.
 *
 * The clock is kept here rather than by the caller, so counting the seconds costs the grid
 * nothing: only the count comes from outside.
 */
export function FetchProgress({ fetched, onStop }: FetchProgressProps) {
  const [elapsed, setElapsed] = useState(0)

  useEffect(() => {
    const started = Date.now()
    const clock = setInterval(() => setElapsed(Date.now() - started), TICK_MS)
    return () => clearInterval(clock)
  }, [])

  return (
    <div className="absolute inset-0 z-10 grid place-items-center">
      <div className="flex flex-col gap-2 rounded-lg border border-line-strong bg-elevated px-4 py-3 shadow-[var(--shadow-overlay)]">
        <span className="flex items-center gap-2 text-sm text-fg">
          <Loader2 size={14} className="shrink-0 animate-spin text-accent" />
          Fetching ({(elapsed / 1000).toFixed(1)}s)
        </span>

        <span className="flex items-center gap-3">
          <span className="text-xs text-muted">Fetched Records:</span>
          <span className="flex-1 text-right font-mono text-sm text-fg">
            {numbers.format(fetched)}
          </span>
          <Button onClick={onStop}>Stop</Button>
        </span>
      </div>
    </div>
  )
}
