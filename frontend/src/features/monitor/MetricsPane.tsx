import { useMemo, useState } from 'react'

import type { AppError } from '@/bridge/errors'
import { Empty } from '@/shared/ui/Empty'
import { ErrorStrip } from '@/shared/ui/ErrorStrip'
import { Loading } from '@/shared/ui/Loading'

import { MetricChart } from './MetricChart'
import { charts, type Reading } from './metrics'

/**
 * The four charts, over the readings taken so far. It draws and does nothing else: the reading is
 * `useMetricsStore`'s, the clock is `useMetricHistory`'s and the arithmetic is `metrics.ts`'s, which
 * is the arrangement every other pane of this tab has.
 *
 * **Two by two rather than four across.** These are read against each other — a traffic chart that
 * climbed while the session count did not is the answer to a different question than one where both
 * did — and four thin strips down a window is one chart nobody can see the shape of.
 *
 * **The crosshair is the pane's and not each panel's**, which is that same reading-against-each-other
 * carried through: the four panels are drawn against one clock, so a pointer on any of them moves the
 * crosshair, the dots, the time and the legend on all four to the one instant. A panel that answered
 * for the moment under the pointer while the three beside it answered for now would be the shape a
 * reader is comparing and the numbers of a different second.
 *
 * See [docs/features/metrics.md](../../../../docs/features/metrics.md).
 */
export function MetricsPane({
  history,
  reading,
  error,
}: {
  history: readonly Reading[]
  reading: boolean
  error: AppError | null
}) {
  const panels = useMemo(() => charts(history), [history])

  // Which reading every panel is reading itself out at, and `null` for a pointer on none of them. It
  // is an index into the clock the four panels share, so nothing on the pane can be a second out of
  // step with anything else on it.
  const [at, setAt] = useState<number | null>(null)

  if (history.length === 0) {
    // Nothing has come back yet, so there is nothing to keep: this is the one moment a failure here
    // empties the pane, because there is no history standing behind it to protect.
    return error !== null ? (
      <Empty title={error.message} hint={error.detail} />
    ) : (
      <Loading label={reading ? 'Asking the server…' : 'Waiting for the first reading…'} />
    )
  }

  return (
    <div className="flex h-full min-h-0 flex-col">
      {/* Two by two where there is room and one column where there is not, and **every panel keeps a
          floor**: a row that divided a short window four ways left a plot fifteen pixels tall, which
          is a chart nobody can read a shape off. Below the floor the grid scrolls, which is the same
          answer the toolbar above gives when its controls will not fit. */}
      <div className="grid min-h-0 flex-1 auto-rows-[minmax(11rem,1fr)] grid-cols-1 gap-2 overflow-auto p-2 lg:grid-cols-2">
        {panels.map((panel) => (
          <MetricChart key={panel.panel} chart={panel} at={at} onAt={setAt} />
        ))}
      </div>

      {/* A failed re-read leaves every chart where it is and says why underneath — what the panes
          beside this one do with the same failure, and what makes a hiccuping server watchable. */}
      {error !== null ? <ErrorStrip error={error} /> : null}
    </div>
  )
}
